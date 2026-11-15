package study

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Nich728/fuxi/internal/dictionary/pinyin"
	"github.com/Nich728/fuxi/internal/jobs"
	"github.com/Nich728/fuxi/internal/study/studydb"
)

// CSV import limits (SYSTEM_DESIGN §12.1, §11).
const (
	MaxCSVBytes = 2 << 20
	MaxCSVRows  = 5000
	csvBatch    = 500
	// maxRowErrors bounds the row errors kept in a job's result.
	maxRowErrors = 50
)

// JobImportCSV is the job kind for CSV imports.
const JobImportCSV = "import.csv"

// CSVRow is one parsed line: hanzi, pinyin, meaning, then optional notes
// and space-separated tags (PRD L5). Pinyin may use tone marks or numbers.
type CSVRow struct {
	Line    int // 1-based line in the file, for error messages
	Hanzi   string
	Pinyin  string
	Meaning string
	Notes   string
	Tags    []string
}

// utf8BOM is the byte order mark Excel puts at the start of "CSV UTF-8" files.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// headerWords mark a first row that names the columns instead of holding data.
var headerWords = map[string]bool{
	"hanzi": true, "characters": true, "character": true, "chinese": true,
	"simplified": true, "word": true, "汉字": true, "汉语": true, "词语": true,
}

// ParseCSV reads class-vocabulary CSV: comma- or tab-separated, an optional
// header row, an optional UTF-8 byte order mark. Blank lines are skipped.
func ParseCSV(data []byte) ([]CSVRow, error) {
	if len(data) > MaxCSVBytes {
		return nil, invalid("file larger than %d bytes", MaxCSVBytes)
	}
	if !utf8.Valid(data) {
		return nil, invalid("file must be UTF-8 text")
	}
	data = bytes.TrimPrefix(data, utf8BOM)
	r := csv.NewReader(bytes.NewReader(data))
	firstLine, _, _ := bytes.Cut(data, []byte("\n"))
	if bytes.Count(firstLine, []byte("\t")) > bytes.Count(firstLine, []byte(",")) {
		r.Comma = '\t' // tab-separated, as Anki and spreadsheets export
	}
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	var rows []CSVRow
	for first := true; ; first = false {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, invalid("CSV: %v", err)
		}
		line, _ := r.FieldPos(0)
		if first && headerWords[strings.ToLower(strings.TrimSpace(rec[0]))] {
			continue
		}
		get := func(i int) string {
			if i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		row := CSVRow{Line: line, Hanzi: get(0), Pinyin: get(1), Meaning: get(2), Notes: get(3)}
		if t := get(4); t != "" {
			row.Tags = strings.Fields(t)
		}
		if row.Hanzi == "" && row.Pinyin == "" && row.Meaning == "" && row.Notes == "" {
			continue
		}
		rows = append(rows, row)
		if len(rows) > MaxCSVRows {
			return nil, invalid("more than %d rows", MaxCSVRows)
		}
	}
	if len(rows) == 0 {
		return nil, invalid("no rows found")
	}
	return rows, nil
}

var numberedSyllable = regexp.MustCompile(`(?i)[a-zü:v]+[1-5]`)

// normalizePinyin turns "ni3 hao3", "ni3hao3" or "lv4" into tone marks;
// pinyin that already has marks (or isn't numbered) is kept as written.
func normalizePinyin(s string) string {
	if !strings.ContainsAny(s, "12345") {
		return s
	}
	sylls := numberedSyllable.FindAllString(s, -1)
	if len(sylls) == 0 {
		return s
	}
	for i, syl := range sylls {
		sylls[i] = strings.NewReplacer("ü", "u:", "v", "u:", "V", "U:").Replace(syl)
	}
	return pinyin.ToMarked(strings.Join(sylls, " "))
}

// importPayload is the import.csv job payload.
type importPayload struct {
	ImportID  uuid.UUID `json:"import_id"`
	DeckID    uuid.UUID `json:"deck_id"`
	StartedAt time.Time `json:"started_at"`
	CSV       string    `json:"csv"`
}

// ImportResult is the import.csv job's result.
type ImportResult struct {
	Imported int        `json:"imported"`
	Skipped  int        `json:"skipped"` // the deck already had that hanzi
	Failed   int        `json:"failed"`
	Errors   []RowError `json:"errors"`
}

// RowError explains why a line wasn't imported.
type RowError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// StartImport checks the deck and the file, then enqueues the import job.
// importID is client-generated, so a retried upload returns the same job.
func (s *Service) StartImport(ctx context.Context, userID, deckID, importID uuid.UUID, data []byte) (int64, error) {
	if importID == uuid.Nil {
		return 0, invalid("import id is required")
	}
	if _, err := s.Deck(ctx, userID, deckID); err != nil {
		return 0, err
	}
	if _, err := ParseCSV(data); err != nil {
		return 0, err
	}
	return jobs.Enqueue(ctx, s.pool, s.clock, jobs.New{
		Kind:      JobImportCSV,
		UserID:    userID,
		DedupeKey: fmt.Sprintf("%s:%s:%s", JobImportCSV, userID, importID),
		Payload:   importPayload{ImportID: importID, DeckID: deckID, StartedAt: s.clock.Now(), CSV: string(data)},
	})
}

// ImportCSVJob runs an import.csv job. Rows are written in batches of 500,
// one transaction each. Note ids are derived from the import id and line,
// so a job that is retried after a crash skips the rows already written.
func (s *Service) ImportCSVJob(ctx context.Context, j jobs.Job) (any, error) {
	var p importPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return nil, jobs.Permanent(fmt.Errorf("decode payload: %w", err))
	}
	deck, err := s.Deck(ctx, j.UserID, p.DeckID)
	if errors.Is(err, ErrNotFound) {
		return nil, jobs.Permanent(errors.New("deck no longer exists"))
	}
	if err != nil {
		return nil, err
	}
	rows, err := ParseCSV([]byte(p.CSV))
	if err != nil {
		return nil, jobs.Permanent(err)
	}
	params, err := s.params.UserParams(ctx, j.UserID)
	if err != nil {
		return nil, err
	}

	res := ImportResult{Errors: []RowError{}}
	rowErr := func(line int, msg string) {
		res.Failed++
		if len(res.Errors) < maxRowErrors {
			res.Errors = append(res.Errors, RowError{Line: line, Message: msg})
		}
	}
	seen := map[string]bool{}
	for start := 0; start < len(rows); start += csvBatch {
		batch := rows[start:min(start+csvBatch, len(rows))]
		notes := make([]NewNote, 0, len(batch))
		for _, r := range batch {
			n, err := s.csvNote(ctx, p, r)
			if errors.Is(err, ErrInvalid) {
				rowErr(r.Line, strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": "))
				continue
			}
			if err != nil {
				return nil, err // e.g. the database is down: retry the job
			}
			if seen[n.Fields.Hanzi] {
				res.Skipped++
				continue
			}
			seen[n.Fields.Hanzi] = true
			notes = append(notes, n)
		}
		err := s.inTx(ctx, func(q *studydb.Queries, _ pgx.Tx) error {
			for _, n := range notes {
				exists, err := q.DeckHasHanzi(ctx, studydb.DeckHasHanziParams{DeckID: deck.ID, UserID: j.UserID, Hanzi: n.Fields.Hanzi})
				if err != nil {
					return fmt.Errorf("check duplicate: %w", err)
				}
				if exists {
					res.Skipped++
					continue
				}
				if _, err := s.insertNote(ctx, q, j.UserID, deck, n, params.Version, s.clock.Now()); err != nil {
					return err
				}
				res.Imported++
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

// csvNote turns a row into a note. Missing pinyin or meaning is filled from
// the dictionary, and the note is linked to the matching entry; the row's
// own pinyin and meaning win, since they are what the class taught.
func (s *Service) csvNote(ctx context.Context, p importPayload, r CSVRow) (NewNote, error) {
	if r.Hanzi == "" {
		return NewNote{}, invalid("first column (hanzi) is empty")
	}
	n := NewNote{
		ID:     rowNoteID(p.ImportID, p.StartedAt, r.Line),
		Kind:   "word",
		Origin: "csv",
		Fields: Fields{Hanzi: r.Hanzi, Pinyin: normalizePinyin(r.Pinyin), Meaning: r.Meaning, Notes: r.Notes},
	}
	tags, err := validTags(r.Tags)
	if err != nil {
		return NewNote{}, err
	}
	n.Tags = tags
	entries, err := s.dict.Headword(ctx, r.Hanzi)
	if err != nil {
		return NewNote{}, fmt.Errorf("look up %s: %w", r.Hanzi, err)
	}
	if e, ok := bestEntry(entries, n.Fields.Pinyin); ok {
		n.EntryID = &e.ID
		if n.Fields.Pinyin == "" {
			n.Fields.Pinyin = e.Pinyin
		}
		if n.Fields.Meaning == "" {
			n.Fields.Meaning = e.Meaning
		}
	}
	if n.Fields.Meaning == "" {
		return NewNote{}, invalid("no meaning given and %s is not in the dictionary", r.Hanzi)
	}
	return n, n.Fields.validate()
}

// bestEntry prefers the entry whose pinyin matches the row's (ignoring
// tones' spelling differences), else the dictionary's first choice.
func bestEntry(entries []DictEntry, rowPinyin string) (DictEntry, bool) {
	if len(entries) == 0 {
		return DictEntry{}, false
	}
	if rowPinyin != "" {
		want := pinyin.NormalizeMarked(rowPinyin)
		for _, e := range entries {
			if pinyin.NormalizeMarked(e.Pinyin) == want {
				return e, true
			}
		}
		want = pinyin.NormalizeToneless(rowPinyin)
		for _, e := range entries {
			if pinyin.NormalizeToneless(e.Pinyin) == want {
				return e, true
			}
		}
	}
	return entries[0], true
}

// rowNoteID is a UUIDv7 whose time is the import's start plus one
// millisecond per line, and whose random bits are a hash of the import id
// and line. It is the same on every run of the job, and imported notes sort
// in file order, so new cards are introduced in the order of the list.
func rowNoteID(importID uuid.UUID, started time.Time, line int) uuid.UUID {
	h := sha256.Sum256(append(importID[:], fmt.Sprintf(":%d", line)...))
	var id uuid.UUID
	copy(id[:], h[:16])
	ms := uint64(started.UnixMilli() + int64(line))
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], ms)
	copy(id[0:6], ts[2:8])
	id[6] = (id[6] & 0x0f) | 0x70 // version 7
	id[8] = (id[8] & 0x3f) | 0x80 // RFC 9562 variant
	return id
}
