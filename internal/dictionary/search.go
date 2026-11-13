package dictionary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Nich728/fuxi/internal/dictionary/dictdb"
	"github.com/Nich728/fuxi/internal/dictionary/pinyin"
)

// Search limits.
const (
	DefaultLimit = 20
	MaxLimit     = 50
	maxQueryLen  = 64
	examplesShow = 10
)

var (
	// ErrBadQuery is returned for an empty or overlong search query.
	ErrBadQuery = errors.New("bad query")
	// ErrNotFound is returned for an unknown entry id.
	ErrNotFound = errors.New("not found")
)

// Entry is a dictionary entry as returned by search.
type Entry struct {
	ID          int64    `json:"id"`
	Simplified  string   `json:"simplified"`
	Traditional string   `json:"traditional"`
	Pinyin      string   `json:"pinyin"`
	Definitions []string `json:"definitions"`
	HSKLevel    *int     `json:"hsk_level"` // 7 means the combined 7–9 band
}

// EntryDetail is one entry with real example sentences (PRD L2).
type EntryDetail struct {
	Entry
	PinyinNumbered string    `json:"pinyin_numbered"`
	Examples       []Example `json:"examples"`
}

// Example is an example sentence containing the entry.
type Example struct {
	ID     int64   `json:"id"`
	Source string  `json:"source"` // 'tatoeba' | 'user' | 'ai'; AI sentences are labelled in the UI (FR-AI-5)
	ZH     string  `json:"zh"`
	EN     *string `json:"en"`
	MaxHSK *int    `json:"max_hsk"`
}

// Service answers dictionary reads. The dictionary is shared content, so
// unlike user data its queries aren't scoped to a user.
type Service struct {
	q *dictdb.Queries
}

// NewService returns a Service over db.
func NewService(db dictdb.DBTX) *Service {
	return &Service{q: dictdb.New(db)}
}

// Search routes the query by its shape (SYSTEM_DESIGN §10):
//   - contains hanzi → prefix match on simplified or traditional;
//   - ASCII with tone digits ("ma3") → numbered pinyin;
//   - anything else → toneless pinyin, merged with English full-text search.
func (s *Service) Search(ctx context.Context, q string, limit int) ([]Entry, error) {
	q = strings.TrimSpace(q)
	if q == "" || len([]rune(q)) > maxQueryLen {
		return nil, fmt.Errorf("%w: query must be 1–%d characters", ErrBadQuery, maxQueryLen)
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	lim := int32(limit)

	switch {
	case hasHan(q):
		rows, err := s.q.SearchByHanzi(ctx, dictdb.SearchByHanziParams{Q: escapeLike(q), Lim: lim})
		return toEntries(rows), err
	case hasToneDigit(q):
		rows, err := s.q.SearchByNumbered(ctx, dictdb.SearchByNumberedParams{Q: pinyin.NormalizeNumbered(q), Lim: lim})
		return toEntries(convert(rows)), err
	default:
		return s.searchPlain(ctx, q, lim)
	}
}

// searchPlain merges toneless-pinyin and English results: exact pinyin
// matches first ("ma" → 妈, 马, 吗), then English matches, then partial pinyin.
func (s *Service) searchPlain(ctx context.Context, q string, lim int32) ([]Entry, error) {
	var pyRows []dictdb.SearchByHanziRow
	if key := pinyin.NormalizeToneless(q); key != "" {
		rows, err := s.q.SearchByToneless(ctx, dictdb.SearchByTonelessParams{Q: escapeLike(key), Lim: lim})
		if err != nil {
			return nil, err
		}
		pyRows = convert(rows)
	}
	enRows, err := s.q.SearchEnglish(ctx, dictdb.SearchEnglishParams{Q: q, Lim: lim})
	if err != nil {
		return nil, err
	}
	var exact, partial []dictdb.SearchByHanziRow
	for _, r := range pyRows {
		if r.Exact {
			exact = append(exact, r)
		} else {
			partial = append(partial, r)
		}
	}
	merged := append(append(exact, convert(enRows)...), partial...)
	seen := map[int64]bool{}
	out := make([]Entry, 0, lim)
	for _, e := range toEntries(merged) {
		if seen[e.ID] || len(out) == int(lim) {
			continue
		}
		seen[e.ID] = true
		out = append(out, e)
	}
	return out, nil
}

// Get returns one entry with up to 10 example sentences, optionally limited
// to sentences at or below maxHSK.
func (s *Service) Get(ctx context.Context, id int64, maxHSK int) (EntryDetail, error) {
	row, err := s.q.GetEntry(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return EntryDetail{}, ErrNotFound
	}
	if err != nil {
		return EntryDetail{}, err
	}
	params := dictdb.ListExamplesParams{EntryID: id, Lim: examplesShow}
	if maxHSK > 0 {
		params.MaxHsk = pgtype.Int2{Int16: int16(maxHSK), Valid: true}
	}
	ex, err := s.q.ListExamples(ctx, params)
	if err != nil {
		return EntryDetail{}, err
	}
	d := EntryDetail{
		Entry: Entry{
			ID: row.ID, Simplified: row.Simplified, Traditional: row.Traditional,
			Pinyin: row.PinyinMarked, Definitions: row.Definitions, HSKLevel: intPtr(row.HskLevel),
		},
		PinyinNumbered: row.PinyinNumbered,
		Examples:       make([]Example, 0, len(ex)),
	}
	for _, e := range ex {
		var en *string
		if e.En.Valid {
			en = &e.En.String
		}
		d.Examples = append(d.Examples, Example{ID: e.ID, Source: e.Source, ZH: e.Zh, EN: en, MaxHSK: intPtr(e.MaxHsk)})
	}
	return d, nil
}

// searchRow is satisfied by every sqlc search row type: they share columns.
type searchRow interface {
	dictdb.SearchByNumberedRow | dictdb.SearchByTonelessRow | dictdb.SearchEnglishRow
}

func convert[T searchRow](rows []T) []dictdb.SearchByHanziRow {
	out := make([]dictdb.SearchByHanziRow, len(rows))
	for i, r := range rows {
		out[i] = dictdb.SearchByHanziRow(r)
	}
	return out
}

func toEntries(rows []dictdb.SearchByHanziRow) []Entry {
	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = Entry{
			ID: r.ID, Simplified: r.Simplified, Traditional: r.Traditional,
			Pinyin: r.PinyinMarked, Definitions: r.Definitions, HSKLevel: intPtr(r.HskLevel),
		}
	}
	return out
}

func intPtr(v pgtype.Int2) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int16)
	return &n
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func hasToneDigit(s string) bool {
	return strings.ContainsAny(s, "12345")
}

// escapeLike makes user input literal inside a LIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
