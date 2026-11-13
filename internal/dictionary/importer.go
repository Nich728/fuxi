package dictionary

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Importer loads open datasets into the content tables (SYSTEM_DESIGN §5).
// Every import is pinned by the SHA-256 of its input, staged with COPY and
// upserted in one transaction, so re-running an import is a no-op and user
// data, which only references dict_entries.id, is never orphaned.
//
// Order matters: CC-CEDICT first, then the HSK list, then Tatoeba. The HSK
// and Tatoeba imports both re-link sentences to entries at the end.
type Importer struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewImporter returns an Importer.
func NewImporter(pool *pgxpool.Pool, log *slog.Logger) *Importer {
	return &Importer{pool: pool, log: log}
}

// Result reports what an import did.
type Result struct {
	Skipped   bool     // the same input was already imported successfully
	Rows      int      // rows inserted or changed
	Unmatched []string // HSK words with no dictionary entry
}

// ErrNoDictionary is returned when an import needs CC-CEDICT loaded first.
var ErrNoDictionary = errors.New("dictionary is empty: import CC-CEDICT first")

// beginRun records a run for (source, sha). It reports false when the same
// input already succeeded, so the caller can stop (§5 step 1).
func (im *Importer) beginRun(ctx context.Context, source, sha string) (int64, bool, error) {
	var id int64
	err := im.pool.QueryRow(ctx, `
		INSERT INTO import_runs (source, file_sha256, status)
		VALUES ($1, $2, 'running')
		ON CONFLICT (file_sha256) DO UPDATE
		    SET status = 'running', started_at = now(), finished_at = NULL, rows_upserted = NULL
		    WHERE import_runs.status <> 'succeeded'
		RETURNING id`, source, sha).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("record import run: %w", err)
	}
	return id, true, nil
}

func (im *Importer) finishRun(ctx context.Context, id int64, runErr error, rows int) {
	status := "succeeded"
	if runErr != nil {
		status = "failed"
	}
	// The run must be marked even if ctx was cancelled.
	ctx = context.WithoutCancel(ctx)
	if _, err := im.pool.Exec(ctx, `
		UPDATE import_runs SET status = $2, rows_upserted = $3, finished_at = now() WHERE id = $1`,
		id, status, rows); err != nil {
		im.log.Error("mark import run", "run_id", id, "err", err)
	}
}

// run wraps one import: pin, run fn in a transaction, record the outcome.
func (im *Importer) run(ctx context.Context, source, sha string, fn func(pgx.Tx) (Result, error)) (Result, error) {
	runID, ok, err := im.beginRun(ctx, source, sha)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		im.log.Info("import skipped: this input was already imported", "source", source, "sha256", sha)
		return Result{Skipped: true}, nil
	}
	var res Result
	err = pgx.BeginFunc(ctx, im.pool, func(tx pgx.Tx) error {
		var err error
		res, err = fn(tx)
		return err
	})
	im.finishRun(ctx, runID, err, res.Rows)
	if err != nil {
		return Result{}, fmt.Errorf("import %s: %w", source, err)
	}
	im.log.Info("import finished", "source", source, "rows", res.Rows, "unmatched", len(res.Unmatched))
	return res, nil
}

// ImportCedict loads a CC-CEDICT file (plain or .gz).
func (im *Importer) ImportCedict(ctx context.Context, path string) (Result, error) {
	sha, err := fileSHA256(path)
	if err != nil {
		return Result{}, err
	}
	return im.run(ctx, "cedict", sha, func(tx pgx.Tx) (Result, error) {
		r, err := openData(path)
		if err != nil {
			return Result{}, err
		}
		defer func() { _ = r.Close() }()
		entries, err := ParseCedict(r)
		if err != nil {
			return Result{}, err
		}
		if _, err := tx.Exec(ctx, `
			CREATE TEMP TABLE cedict_stage (
			    source_key text, simplified text, traditional text, pinyin_numbered text,
			    pinyin_marked text, pinyin_toneless text, definitions text[], secondary boolean
			) ON COMMIT DROP`); err != nil {
			return Result{}, err
		}
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"cedict_stage"},
			[]string{"source_key", "simplified", "traditional", "pinyin_numbered", "pinyin_marked", "pinyin_toneless", "definitions", "secondary"},
			pgx.CopyFromSlice(len(entries), func(i int) ([]any, error) {
				e := entries[i]
				return []any{e.SourceKey, e.Simplified, e.Traditional, e.PinyinNumbered, e.PinyinMarked, e.PinyinToneless, e.Definitions, e.Secondary}, nil
			}))
		if err != nil {
			return Result{}, fmt.Errorf("copy cedict: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO dict_entries (source, source_key, simplified, traditional,
			                          pinyin_numbered, pinyin_marked, pinyin_toneless, definitions, secondary)
			SELECT 'cedict', source_key, simplified, traditional,
			       pinyin_numbered, pinyin_marked, pinyin_toneless, definitions, secondary
			FROM cedict_stage
			ON CONFLICT (source, source_key) DO UPDATE SET
			    simplified = EXCLUDED.simplified,
			    traditional = EXCLUDED.traditional,
			    pinyin_numbered = EXCLUDED.pinyin_numbered,
			    pinyin_marked = EXCLUDED.pinyin_marked,
			    pinyin_toneless = EXCLUDED.pinyin_toneless,
			    definitions = EXCLUDED.definitions,
			    secondary = EXCLUDED.secondary
			WHERE (dict_entries.pinyin_marked, dict_entries.pinyin_toneless, dict_entries.definitions, dict_entries.secondary)
			      IS DISTINCT FROM (EXCLUDED.pinyin_marked, EXCLUDED.pinyin_toneless, EXCLUDED.definitions, EXCLUDED.secondary)`)
		if err != nil {
			return Result{}, fmt.Errorf("upsert cedict: %w", err)
		}
		return Result{Rows: int(tag.RowsAffected())}, nil
	})
}

// ImportHSK tags dict_entries.hsk_level from the HSK 3.0 examination
// syllabus (§5 step 5). Words on no list keep hsk_level = NULL.
func (im *Importer) ImportHSK(ctx context.Context, path string) (Result, error) {
	sha, err := fileSHA256(path)
	if err != nil {
		return Result{}, err
	}
	return im.run(ctx, "hsk", sha, func(tx pgx.Tx) (Result, error) {
		if err := requireDictionary(ctx, tx); err != nil {
			return Result{}, err
		}
		r, err := openData(path)
		if err != nil {
			return Result{}, err
		}
		defer func() { _ = r.Close() }()
		words, err := ParseHSK(r)
		if err != nil {
			return Result{}, err
		}
		rows, err := tx.Query(ctx, `
			SELECT id, simplified, pinyin_marked, pinyin_toneless, secondary FROM dict_entries`)
		if err != nil {
			return Result{}, err
		}
		cands, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (HSKCandidate, error) {
			var c HSKCandidate
			err := row.Scan(&c.ID, &c.Simplified, &c.Marked, &c.Toneless, &c.Secondary)
			return c, err
		})
		if err != nil {
			return Result{}, err
		}
		tags, unmatched := MatchHSK(cands, words)

		if _, err := tx.Exec(ctx, `
			CREATE TEMP TABLE hsk_stage (entry_id bigint, level smallint) ON COMMIT DROP`); err != nil {
			return Result{}, err
		}
		ids := make([]int64, 0, len(tags))
		for id := range tags {
			ids = append(ids, id)
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"hsk_stage"}, []string{"entry_id", "level"},
			pgx.CopyFromSlice(len(ids), func(i int) ([]any, error) {
				return []any{ids[i], int16(tags[ids[i]])}, nil
			})); err != nil {
			return Result{}, fmt.Errorf("copy hsk: %w", err)
		}
		// Rebuild from scratch, so a newer list can also lower or remove levels.
		if _, err := tx.Exec(ctx, `
			UPDATE dict_entries d SET hsk_level = NULL
			WHERE hsk_level IS NOT NULL AND NOT EXISTS (SELECT 1 FROM hsk_stage s WHERE s.entry_id = d.id)`); err != nil {
			return Result{}, err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE dict_entries d SET hsk_level = s.level
			FROM hsk_stage s
			WHERE d.id = s.entry_id AND d.hsk_level IS DISTINCT FROM s.level`)
		if err != nil {
			return Result{}, fmt.Errorf("tag hsk: %w", err)
		}
		if err := relink(ctx, tx); err != nil {
			return Result{}, err
		}
		return Result{Rows: int(tag.RowsAffected()), Unmatched: unmatched}, nil
	})
}

// ImportTatoeba loads Mandarin sentences with English translations
// (cmn_sentences.tsv, eng_sentences.tsv, cmn-eng_links.tsv; plain or .bz2),
// then links every sentence to the dictionary words it contains (§5 step 4).
func (im *Importer) ImportTatoeba(ctx context.Context, cmnPath, engPath, linksPath string) (Result, error) {
	sha, err := combinedSHA256(cmnPath, engPath, linksPath)
	if err != nil {
		return Result{}, err
	}
	return im.run(ctx, "tatoeba", sha, func(tx pgx.Tx) (Result, error) {
		if err := requireDictionary(ctx, tx); err != nil {
			return Result{}, err
		}
		sentences, err := readTatoeba(cmnPath, engPath, linksPath)
		if err != nil {
			return Result{}, err
		}
		if _, err := tx.Exec(ctx, `
			CREATE TEMP TABLE tatoeba_stage (source_id text, zh text, en text) ON COMMIT DROP`); err != nil {
			return Result{}, err
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"tatoeba_stage"}, []string{"source_id", "zh", "en"},
			pgx.CopyFromSlice(len(sentences), func(i int) ([]any, error) {
				s := sentences[i]
				var en any
				if s.EN != "" {
					en = s.EN
				}
				return []any{s.ID, s.ZH, en}, nil
			})); err != nil {
			return Result{}, fmt.Errorf("copy tatoeba: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO sentences (source, source_id, zh, en)
			SELECT 'tatoeba', source_id, zh, en FROM tatoeba_stage
			ON CONFLICT (source, source_id) DO UPDATE SET zh = EXCLUDED.zh, en = EXCLUDED.en
			WHERE (sentences.zh, sentences.en) IS DISTINCT FROM (EXCLUDED.zh, EXCLUDED.en)`)
		if err != nil {
			return Result{}, fmt.Errorf("upsert sentences: %w", err)
		}
		if err := relink(ctx, tx); err != nil {
			return Result{}, err
		}
		return Result{Rows: int(tag.RowsAffected())}, nil
	})
}

func readTatoeba(cmnPath, engPath, linksPath string) ([]TatoebaSentence, error) {
	read := func(path string, fn func(io.Reader) error) error {
		r, err := openData(path)
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()
		return fn(r)
	}
	var links map[string][]string
	if err := read(linksPath, func(r io.Reader) (err error) { links, err = ReadTatoebaLinks(r); return }); err != nil {
		return nil, fmt.Errorf("read links: %w", err)
	}
	var cmn map[string]string
	if err := read(cmnPath, func(r io.Reader) (err error) { cmn, err = ReadTatoebaSentences(r, nil); return }); err != nil {
		return nil, fmt.Errorf("read cmn sentences: %w", err)
	}
	keep := map[string]bool{}
	for _, ids := range links {
		for _, id := range ids {
			keep[id] = true
		}
	}
	var eng map[string]string
	if err := read(engPath, func(r io.Reader) (err error) { eng, err = ReadTatoebaSentences(r, keep); return }); err != nil {
		return nil, fmt.Errorf("read eng sentences: %w", err)
	}
	return JoinTatoeba(cmn, links, eng), nil
}

func requireDictionary(ctx context.Context, tx pgx.Tx) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM dict_entries)`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNoDictionary
	}
	return nil
}

// relink rebuilds sentence_words from scratch, then derives each sentence's
// max_hsk and each entry's frequency_rank from the links.
//
// max_hsk is the hardest HSK level among a sentence's words, ignoring names
// and pointer entries (names such as 汤姆 aren't on any list). A sentence with a word on no
// list gets NULL, so level filters leave it out.
//
// frequency_rank ranks entries by how many sentences use them (1 = most).
// It's a stand-in until a proper word-frequency list is chosen.
func relink(ctx context.Context, tx pgx.Tx) error {
	rows, err := tx.Query(ctx, `
		SELECT id, simplified, traditional, coalesce(hsk_level, 0), secondary FROM dict_entries`)
	if err != nil {
		return err
	}
	var entries []SegmentEntry
	for rows.Next() {
		var e SegmentEntry
		var level int16
		if err := rows.Scan(&e.ID, &e.Simplified, &e.Traditional, &level, &e.Secondary); err != nil {
			rows.Close()
			return err
		}
		e.HSKLevel = int(level)
		entries = append(entries, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	seg := NewSegmenter(entries)

	type link struct{ sentence, entry int64 }
	var links []link
	srows, err := tx.Query(ctx, `SELECT id, zh FROM sentences`)
	if err != nil {
		return err
	}
	for srows.Next() {
		var id int64
		var zh string
		if err := srows.Scan(&id, &zh); err != nil {
			srows.Close()
			return err
		}
		for _, e := range seg.Segment(zh) {
			links = append(links, link{id, e})
		}
	}
	srows.Close()
	if err := srows.Err(); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM sentence_words`); err != nil {
		return err
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"sentence_words"}, []string{"sentence_id", "entry_id"},
		pgx.CopyFromSlice(len(links), func(i int) ([]any, error) {
			return []any{links[i].sentence, links[i].entry}, nil
		})); err != nil {
		return fmt.Errorf("copy sentence_words: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sentences s SET max_hsk = x.m
		FROM (
		    SELECT s2.id,
		           CASE WHEN bool_or(d.id IS NOT NULL AND d.hsk_level IS NULL AND NOT d.secondary)
		                THEN NULL
		                ELSE max(d.hsk_level) END AS m
		    FROM sentences s2
		    LEFT JOIN sentence_words sw ON sw.sentence_id = s2.id
		    LEFT JOIN dict_entries d ON d.id = sw.entry_id
		    GROUP BY s2.id
		) x
		WHERE s.id = x.id AND s.max_hsk IS DISTINCT FROM x.m`); err != nil {
		return fmt.Errorf("compute max_hsk: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE dict_entries d SET frequency_rank = r.rank
		FROM (
		    SELECT e.id, CASE WHEN count(sw.entry_id) = 0 THEN NULL
		                      ELSE row_number() OVER (ORDER BY count(sw.entry_id) DESC, e.id) END AS rank
		    FROM dict_entries e LEFT JOIN sentence_words sw ON sw.entry_id = e.id
		    GROUP BY e.id
		) r
		WHERE d.id = r.id AND d.frequency_rank IS DISTINCT FROM r.rank`); err != nil {
		return fmt.Errorf("compute frequency_rank: %w", err)
	}
	return nil
}
