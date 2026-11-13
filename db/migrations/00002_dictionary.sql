-- Shared, read-only content: dictionary entries, example sentences and
-- import bookkeeping (SYSTEM_DESIGN §4.2, §5). Owned by internal/dictionary.

-- +goose Up

-- array_to_string is only STABLE, but generated columns need IMMUTABLE
-- functions. Joining a text[] with a fixed separator is deterministic.
-- +goose StatementBegin
CREATE FUNCTION dict_join_definitions(defs text[]) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    RETURN array_to_string(defs, ' ');
-- +goose StatementEnd

CREATE TABLE dict_entries (
    id               bigserial PRIMARY KEY,
    source           text NOT NULL,                 -- 'cedict'
    source_key       text NOT NULL,                 -- 'trad|simp|pinyin', stable across imports
    simplified       text NOT NULL,
    traditional      text NOT NULL,
    pinyin_numbered  text NOT NULL,                 -- 'ci2 dian3'
    pinyin_marked    text NOT NULL,                 -- 'cídiǎn'
    pinyin_toneless  text NOT NULL,                 -- 'cidian' (lowercase, no spaces, ü written u)
    definitions      text[] NOT NULL,
    -- Names, surnames and pointer entries ("variant of …", "used in …", "see …").
    -- They rank below ordinary words and lose ties when a headword has several entries.
    secondary        boolean NOT NULL DEFAULT false,
    hsk_level        smallint CHECK (hsk_level BETWEEN 1 AND 7),  -- 7 = the combined 7–9 band
    frequency_rank   integer,                        -- 1 = most frequent (§5 step 4)
    search_en        tsvector GENERATED ALWAYS AS
                       (to_tsvector('english', dict_join_definitions(definitions))) STORED,
    UNIQUE (source, source_key)
);
CREATE INDEX dict_simplified_idx  ON dict_entries (simplified text_pattern_ops);
CREATE INDEX dict_traditional_idx ON dict_entries (traditional text_pattern_ops);
CREATE INDEX dict_toneless_idx    ON dict_entries (pinyin_toneless text_pattern_ops);
CREATE INDEX dict_toneless_trgm   ON dict_entries USING gin (pinyin_toneless gin_trgm_ops);
-- Numbered pinyin is matched with spaces removed, so 'ci2dian3' and 'ci2 dian3' both work.
CREATE INDEX dict_numbered_idx    ON dict_entries ((lower(replace(pinyin_numbered, ' ', ''))));
CREATE INDEX dict_en_fts          ON dict_entries USING gin (search_en);

CREATE TABLE sentences (
    id         bigserial PRIMARY KEY,
    source     text NOT NULL,                       -- 'tatoeba' | 'user' | 'ai'
    source_id  text,
    zh         text NOT NULL,
    en         text,
    trust      smallint NOT NULL DEFAULT 0,         -- -1 hidden, 0 unknown, 1 reviewed
    max_hsk    smallint,                            -- hardest word level, for level filtering
    UNIQUE (source, source_id)
);

CREATE TABLE sentence_words (
    sentence_id bigint NOT NULL REFERENCES sentences(id) ON DELETE CASCADE,
    entry_id    bigint NOT NULL REFERENCES dict_entries(id),
    PRIMARY KEY (entry_id, sentence_id)
);
CREATE INDEX sentence_words_sentence_idx ON sentence_words (sentence_id);

CREATE TABLE import_runs (
    id            bigserial PRIMARY KEY,
    source        text NOT NULL,
    file_sha256   text NOT NULL UNIQUE,
    status        text NOT NULL,                    -- running | succeeded | failed
    rows_upserted integer,
    started_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz
);

-- +goose Down
DROP TABLE import_runs;
DROP TABLE sentence_words;
DROP TABLE sentences;
DROP TABLE dict_entries;
DROP FUNCTION dict_join_definitions(text[]);
