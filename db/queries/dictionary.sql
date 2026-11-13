-- Read queries for internal/dictionary (SYSTEM_DESIGN §10, "Search routing").
-- Ranking everywhere: exact match, then ordinary words before names and
-- pointer entries (dict_entries.secondary) that aren't on the HSK list (so
-- 中文 and 中国, which CC-CEDICT capitalises, aren't demoted), then HSK level,
-- then frequency.

-- name: SearchByHanzi :many
-- Prefix match on simplified or traditional characters.
SELECT id, simplified, traditional, pinyin_marked, definitions, hsk_level, frequency_rank,
       (simplified = sqlc.arg(q)::text OR traditional = sqlc.arg(q)::text)::bool AS exact
FROM dict_entries
WHERE simplified LIKE sqlc.arg(q)::text || '%' OR traditional LIKE sqlc.arg(q)::text || '%'
ORDER BY exact DESC, (secondary AND hsk_level IS NULL), hsk_level ASC NULLS LAST, frequency_rank ASC NULLS LAST, id
LIMIT sqlc.arg(lim);

-- name: SearchByNumbered :many
-- Numbered pinyin such as 'ma3' or 'ci2dian3'; the key has spaces removed and ü written u:.
SELECT id, simplified, traditional, pinyin_marked, definitions, hsk_level, frequency_rank,
       true::bool AS exact
FROM dict_entries
WHERE lower(replace(pinyin_numbered, ' ', '')) = sqlc.arg(q)::text
ORDER BY (secondary AND hsk_level IS NULL), hsk_level ASC NULLS LAST, frequency_rank ASC NULLS LAST, id
LIMIT sqlc.arg(lim);

-- name: SearchByToneless :many
-- Toneless pinyin: exact, then prefix, then trigram-similar ('zhidao' finds 知道).
SELECT id, simplified, traditional, pinyin_marked, definitions, hsk_level, frequency_rank,
       (pinyin_toneless = sqlc.arg(q)::text)::bool AS exact
FROM dict_entries
WHERE pinyin_toneless LIKE sqlc.arg(q)::text || '%' OR pinyin_toneless % sqlc.arg(q)::text
ORDER BY exact DESC,
         (pinyin_toneless LIKE sqlc.arg(q)::text || '%') DESC,
         (secondary AND hsk_level IS NULL),
         hsk_level ASC NULLS LAST, frequency_rank ASC NULLS LAST, id
LIMIT sqlc.arg(lim);

-- name: SearchEnglish :many
-- English full-text search over definitions. A sense equal to the query
-- (or to "to <query>" for verbs) counts as exact. CC-CEDICT packs several
-- senses into one definition with ";" ("to know; to become aware of").
SELECT id, simplified, traditional, pinyin_marked, definitions, hsk_level, frequency_rank,
       EXISTS (
           SELECT 1 FROM unnest(definitions) AS d(def), regexp_split_to_table(d.def, ';\s*') AS s(sense)
           WHERE lower(s.sense) IN (lower(sqlc.arg(q)::text), 'to ' || lower(sqlc.arg(q)::text))
       )::bool AS exact
FROM dict_entries
WHERE search_en @@ plainto_tsquery('english', sqlc.arg(q)::text)
ORDER BY exact DESC, (secondary AND hsk_level IS NULL), hsk_level ASC NULLS LAST, frequency_rank ASC NULLS LAST,
         ts_rank(search_en, plainto_tsquery('english', sqlc.arg(q)::text)) DESC, id
LIMIT sqlc.arg(lim);

-- name: GetEntry :one
SELECT id, simplified, traditional, pinyin_numbered, pinyin_marked, definitions, hsk_level, frequency_rank
FROM dict_entries
WHERE id = $1;

-- name: ListExamples :many
-- Example sentences containing an entry, easiest and shortest first.
-- Hidden sentences (trust < 0) are never shown.
SELECT s.id, s.source, s.zh, s.en, s.max_hsk
FROM sentence_words sw
JOIN sentences s ON s.id = sw.sentence_id
WHERE sw.entry_id = sqlc.arg(entry_id) AND s.trust >= 0
  AND (sqlc.narg(max_hsk)::smallint IS NULL OR s.max_hsk <= sqlc.narg(max_hsk)::smallint)
ORDER BY s.max_hsk ASC NULLS LAST, char_length(s.zh), s.id
LIMIT sqlc.arg(lim);
