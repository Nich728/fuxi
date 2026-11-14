package dictionary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
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
	db dictdb.DBTX
	q  *dictdb.Queries

	// The phrase segmenter is built from every entry (about 125,000), so it
	// is cached and rebuilt only when an import has finished since.
	segMu      sync.Mutex
	seg        *Segmenter
	segVersion time.Time
}

// NewService returns a Service over db.
func NewService(db dictdb.DBTX) *Service {
	return &Service{db: db, q: dictdb.New(db)}
}

// SearchResult is the answer to a search.
type SearchResult struct {
	Results []Entry `json:"results"`
	// Segmented is true when a Chinese phrase matched no dictionary entry and
	// Results are instead the words it contains, in phrase order (我喜欢 → 我, 喜欢).
	Segmented bool `json:"segmented"`
}

// Search routes the query by its shape (SYSTEM_DESIGN §10):
//   - contains hanzi → prefix match on simplified or traditional; if nothing
//     matches, the words inside the phrase;
//   - ASCII with tone digits ("ma3") → numbered pinyin;
//   - anything else → toneless pinyin, merged with English full-text search.
func (s *Service) Search(ctx context.Context, q string, limit int) (SearchResult, error) {
	q = strings.TrimSpace(q)
	if q == "" || len([]rune(q)) > maxQueryLen {
		return SearchResult{}, fmt.Errorf("%w: query must be 1–%d characters", ErrBadQuery, maxQueryLen)
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	lim := int32(limit)

	var (
		entries []Entry
		err     error
	)
	switch {
	case hasHan(q):
		var rows []dictdb.SearchByHanziRow
		rows, err = s.q.SearchByHanzi(ctx, dictdb.SearchByHanziParams{Q: escapeLike(q), Lim: lim})
		if err == nil && len(rows) == 0 {
			return s.searchPhrase(ctx, q, limit)
		}
		entries = toEntries(rows)
	case hasToneDigit(q):
		var rows []dictdb.SearchByNumberedRow
		rows, err = s.q.SearchByNumbered(ctx, dictdb.SearchByNumberedParams{Q: pinyin.NormalizeNumbered(q), Lim: lim})
		entries = toEntries(convert(rows))
	default:
		entries, err = s.searchPlain(ctx, q, lim)
	}
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Results: entries}, nil
}

// searchPhrase splits a phrase that isn't itself a dictionary word into the
// words it contains, using the same greedy longest match as sentence linking
// (SYSTEM_DESIGN §5 step 4). Characters the dictionary doesn't know are skipped.
func (s *Service) searchPhrase(ctx context.Context, q string, limit int) (SearchResult, error) {
	seg, err := s.segmenter(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	ids := seg.Segment(q)
	if len(ids) > limit {
		ids = ids[:limit]
	}
	if len(ids) == 0 {
		return SearchResult{Results: []Entry{}}, nil
	}
	rows, err := s.q.GetEntriesByIDs(ctx, ids)
	if err != nil {
		return SearchResult{}, err
	}
	byID := make(map[int64]dictdb.SearchByHanziRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = dictdb.SearchByHanziRow(r)
	}
	ordered := make([]dictdb.SearchByHanziRow, 0, len(ids))
	for _, id := range ids {
		if r, ok := byID[id]; ok {
			ordered = append(ordered, r)
		}
	}
	return SearchResult{Results: toEntries(ordered), Segmented: true}, nil
}

// segmenter returns the cached Segmenter, rebuilding it if an import has
// finished since it was built. Checking costs one small query.
func (s *Service) segmenter(ctx context.Context) (*Segmenter, error) {
	v, err := s.q.DictionaryVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("dictionary version: %w", err)
	}
	s.segMu.Lock()
	defer s.segMu.Unlock()
	if s.seg != nil && s.segVersion.Equal(v.Time) {
		return s.seg, nil
	}
	seg, err := loadSegmenter(ctx, s.db)
	if err != nil {
		return nil, err
	}
	s.seg, s.segVersion = seg, v.Time
	return seg, nil
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
	dictdb.SearchByNumberedRow | dictdb.SearchByTonelessRow | dictdb.SearchEnglishRow | dictdb.GetEntriesByIDsRow
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
