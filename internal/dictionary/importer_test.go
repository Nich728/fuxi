package dictionary_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nich728/fuxi/internal/dictionary"
	"github.com/Nich728/fuxi/internal/platform/testdb"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func td(name string) string { return filepath.Join("testdata", name) }

// TestImportAndSearch covers M1's "done when" (SYSTEM_DESIGN §14):
// "ma" → 妈 / 马 / 吗, and re-importing is a no-op.
func TestImportAndSearch(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Start(t, true)
	im := dictionary.NewImporter(pool, quiet)

	// HSK and Tatoeba need the dictionary first.
	if _, err := im.ImportHSK(ctx, td("hsk_sample.tsv")); err == nil {
		t.Fatal("ImportHSK on an empty dictionary should fail")
	}

	res, err := im.ImportCedict(ctx, td("cedict_sample.u8"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows != 27 {
		t.Fatalf("cedict rows = %d, want 27", res.Rows)
	}
	idsBefore := entryIDs(t, pool)

	t.Run("same file again is skipped", func(t *testing.T) {
		res, err := im.ImportCedict(ctx, td("cedict_sample.u8"))
		if err != nil || !res.Skipped {
			t.Fatalf("re-import: res=%+v err=%v, want skipped", res, err)
		}
	})
	t.Run("same content in a new file changes nothing", func(t *testing.T) {
		src, err := os.ReadFile(td("cedict_sample.u8"))
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(t.TempDir(), "cedict_new_release.u8")
		if err := os.WriteFile(p, append([]byte("# a newer release header\n"), src...), 0o600); err != nil {
			t.Fatal(err)
		}
		res, err := im.ImportCedict(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		if res.Skipped || res.Rows != 0 {
			t.Fatalf("res = %+v, want a run that changes 0 rows", res)
		}
		if got := entryIDs(t, pool); !reflect.DeepEqual(got, idsBefore) {
			t.Fatal("entry ids changed after re-import: user notes would be orphaned")
		}
	})

	res, err = im.ImportHSK(ctx, td("hsk_sample.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Unmatched, []string{"没有词"}) {
		t.Errorf("unmatched = %v, want [没有词]", res.Unmatched)
	}
	levels := []struct {
		key   string // source_key
		level any    // int16 or nil
	}{
		{"了|了|le5", int16(1)},
		{"了|了|liao3", int16(5)},
		{"馬|马|ma3", int16(3)},
		{"罵|骂|ma4", int16(7)},
		{"女兒|女儿|nu:3 er2", int16(2)},
		{"媽|妈|ma1", nil}, // only 妈妈 is on the list
		{"嗎|吗|ma2", nil}, // the syllabus lists 吗 ma, not má
		{"傌|骂|ma4", nil}, // a variant loses to the ordinary entry
		{"嗎|吗|ma5", int16(1)},
		{"中文|中文|Zhong1 wen2", int16(1)}, // capitalised entries can still be tagged
	}
	for _, l := range levels {
		var got *int16
		if err := pool.QueryRow(ctx, `SELECT hsk_level FROM dict_entries WHERE source_key = $1`, l.key).Scan(&got); err != nil {
			t.Fatalf("%s: %v", l.key, err)
		}
		switch want := l.level.(type) {
		case nil:
			if got != nil {
				t.Errorf("%s hsk_level = %d, want NULL", l.key, *got)
			}
		case int16:
			if got == nil || *got != want {
				t.Errorf("%s hsk_level = %v, want %d", l.key, got, want)
			}
		}
	}

	res, err = im.ImportTatoeba(ctx, td("cmn_sentences.tsv"), td("eng_sentences.tsv"), td("cmn-eng_links.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows != 4 {
		t.Fatalf("tatoeba rows = %d, want 4", res.Rows)
	}
	if res, err := im.ImportTatoeba(ctx, td("cmn_sentences.tsv"), td("eng_sentences.tsv"), td("cmn-eng_links.tsv")); err != nil || !res.Skipped {
		t.Fatalf("tatoeba re-import: res=%+v err=%v, want skipped", res, err)
	}

	t.Run("sentences are linked to their words", func(t *testing.T) {
		var words []string
		err := pool.QueryRow(ctx, `
			SELECT array_agg(d.simplified ORDER BY d.simplified)
			FROM sentence_words sw JOIN sentences s ON s.id = sw.sentence_id
			JOIN dict_entries d ON d.id = sw.entry_id
			WHERE s.source_id = '1002'`).Scan(&words)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"你", "吗", "汤姆", "认识"}; !slices.Equal(sortedCopy(words), sortedCopy(want)) {
			t.Errorf("words in 1002 = %v, want %v", words, want)
		}
		var maxHSK *int16
		if err := pool.QueryRow(ctx, `SELECT max_hsk FROM sentences WHERE source_id = '1002'`).Scan(&maxHSK); err != nil {
			t.Fatal(err)
		}
		if maxHSK == nil || *maxHSK != 1 {
			t.Errorf("max_hsk of 1002 = %v, want 1 (the name 汤姆 is ignored)", maxHSK)
		}
		var rank *int32
		if err := pool.QueryRow(ctx, `SELECT frequency_rank FROM dict_entries WHERE source_key = '我|我|wo3'`).Scan(&rank); err != nil {
			t.Fatal(err)
		}
		if rank == nil || *rank != 1 {
			t.Errorf("frequency_rank of 我 = %v, want 1 (it's in the most sentences)", rank)
		}
	})

	srv := httptest.NewServer(router(pool))
	defer srv.Close()

	searches := []struct {
		q        string
		first    string   // simplified form of the first result
		contains []string // simplified forms that must appear
	}{
		{"ma", "吗", []string{"妈", "马", "吗"}},
		{"Ma", "吗", []string{"妈", "马", "吗"}},
		{"ma3", "马", []string{"马"}},
		{"妈", "妈", []string{"妈妈"}},
		{"媽", "妈", nil}, // traditional input
		{"zhidao", "知道", nil},
		{"zhi dao", "知道", nil},
		{"zhīdào", "知道", nil},
		{"ci2 dian3", "词典", nil},
		{"lv4", "绿", nil},
		{"know", "知道", []string{"认识"}},
		{"dictionary", "词典", nil},
	}
	for _, s := range searches {
		t.Run("search "+s.q, func(t *testing.T) {
			var body struct {
				Results []dictionary.Entry `json:"results"`
			}
			getJSON(t, srv.URL+"/v1/dict/search?q="+url.QueryEscape(s.q), http.StatusOK, &body)
			if len(body.Results) == 0 {
				t.Fatal("no results")
			}
			got := make([]string, len(body.Results))
			for i, r := range body.Results {
				got[i] = r.Simplified
			}
			if got[0] != s.first {
				t.Errorf("first result = %s, want %s (all: %v)", got[0], s.first, got)
			}
			for _, w := range s.contains {
				if !slices.Contains(got, w) {
					t.Errorf("results %v missing %s", got, w)
				}
			}
		})
	}

	t.Run("names and variants rank below ordinary words", func(t *testing.T) {
		var body struct {
			Results []dictionary.Entry `json:"results"`
		}
		getJSON(t, srv.URL+"/v1/dict/search?q=ma", http.StatusOK, &body)
		pos := map[string]int{}
		for i, r := range body.Results {
			if _, seen := pos[r.Definitions[0]]; !seen {
				pos[r.Definitions[0]] = i
			}
		}
		for _, demoted := range []string{"surname Ma", "variant of 罵|骂[ma4]"} {
			if pos[demoted] < pos["ma"] { // "ma" is 妈's first definition
				t.Errorf("%q ranked above 妈: %v", demoted, pos)
			}
		}
	})

	t.Run("bad queries", func(t *testing.T) {
		for _, q := range []string{"", "%20%20", "?limit=0&q=ma", "?limit=51&q=ma"} {
			u := srv.URL + "/v1/dict/search?q=" + q
			if q != "" && q[0] == '?' {
				u = srv.URL + "/v1/dict/search" + q
			}
			resp, err := http.Get(u)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%s: status %d, want 400", u, resp.StatusCode)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("%s: content-type %q", u, ct)
			}
		}
	})

	t.Run("LIKE wildcards are literal", func(t *testing.T) {
		var body struct {
			Results []dictionary.Entry `json:"results"`
		}
		getJSON(t, srv.URL+"/v1/dict/search?q="+url.QueryEscape("%"), http.StatusOK, &body)
		if len(body.Results) != 0 {
			t.Errorf("'%%' matched %d entries", len(body.Results))
		}
	})

	t.Run("entry with examples", func(t *testing.T) {
		var id int64
		if err := pool.QueryRow(ctx, `SELECT id FROM dict_entries WHERE simplified = '知道'`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		var d dictionary.EntryDetail
		getJSON(t, srv.URL+"/v1/dict/entries/"+itoa(id), http.StatusOK, &d)
		if d.Pinyin != "zhīdao" || d.HSKLevel == nil || *d.HSKLevel != 1 {
			t.Errorf("entry = %+v", d.Entry)
		}
		if len(d.Examples) != 1 || d.Examples[0].ZH != "他不知道。" || d.Examples[0].EN == nil || *d.Examples[0].EN != "He does not know." {
			t.Errorf("examples = %+v", d.Examples)
		}
		getJSON(t, srv.URL+"/v1/dict/entries/999999", http.StatusNotFound, nil)
		getJSON(t, srv.URL+"/v1/dict/entries/abc", http.StatusNotFound, nil)
	})
}

func router(pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()
	r.Route("/v1", dictionary.NewHandler(dictionary.NewService(pool), quiet).Routes)
	return r
}

func getJSON(t *testing.T, u string, wantStatus int, v any) {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != wantStatus {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: status %d, want %d: %s", u, resp.StatusCode, wantStatus, b)
	}
	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatal(err)
		}
	}
}

func entryIDs(t *testing.T, pool *pgxpool.Pool) map[string]int64 {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT source_key, id FROM dict_entries`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var k string
		var id int64
		if err := rows.Scan(&k, &id); err != nil {
			t.Fatal(err)
		}
		out[k] = id
	}
	return out
}

func sortedCopy(s []string) []string {
	c := slices.Clone(s)
	slices.Sort(c)
	return c
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
