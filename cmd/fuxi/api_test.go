package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nich728/fuxi/internal/dictionary"
	"github.com/Nich728/fuxi/internal/identity"
	"github.com/Nich728/fuxi/internal/jobs"
	"github.com/Nich728/fuxi/internal/platform/clock"
	"github.com/Nich728/fuxi/internal/platform/testdb"
	"github.com/Nich728/fuxi/internal/scheduling"
	"github.com/Nich728/fuxi/internal/study"
)

// testAPI is the real router on a throwaway database, with dev sign-in:
// each request names its user in the X-Fuxi-Dev-Email header.
type testAPI struct {
	t      *testing.T
	pool   *pgxpool.Pool
	clock  *clock.Fake
	router http.Handler
	log    *slog.Logger
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	pool := testdb.Start(t, true)
	// 09:00 in Jakarta, the default time zone.
	clk := clock.NewFake(time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC))
	log := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return &testAPI{t: t, pool: pool, clock: clk, log: log,
		router: newRouter(routerDeps{pool: pool, log: log, auth: identity.Dev{}, clock: clk, ownerEmail: "owner@example.com"})}
}

// testWriter sends server logs to the test's output, shown when it fails.
type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}

type response struct {
	status int
	body   []byte
}

func (r response) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		t.Fatalf("decode %s: %v", r.body, err)
	}
	return m
}

// call sends a request as email ("" = not signed in). body may be a string
// (sent as is) or a value (sent as JSON).
func (a *testAPI) call(email, method, path string, body any) response {
	a.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		j, err := json.Marshal(b)
		if err != nil {
			a.t.Fatal(err)
		}
		rd = bytes.NewReader(j)
	}
	req := httptest.NewRequest(method, path, rd)
	if email != "" {
		req.Header.Set(identity.DevHeader, email)
	}
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return response{status: rec.Code, body: rec.Body.Bytes()}
}

// must calls and fails the test unless the status matches.
func (a *testAPI) must(want int, email, method, path string, body any) map[string]any {
	a.t.Helper()
	r := a.call(email, method, path, body)
	if r.status != want {
		a.t.Fatalf("%s %s = %d, want %d: %s", method, path, r.status, want, r.body)
	}
	if len(r.body) == 0 {
		return nil
	}
	return r.json(a.t)
}

func (a *testAPI) addEntry(simp, numbered, marked string, defs ...string) int64 {
	a.t.Helper()
	var id int64
	err := a.pool.QueryRow(context.Background(), `
		INSERT INTO dict_entries (source, source_key, simplified, traditional, pinyin_numbered, pinyin_marked, pinyin_toneless, definitions)
		VALUES ('cedict', $1 || '|' || $2, $1, $1, $2, $3, $4, $5) RETURNING id`,
		simp, numbered, marked, strings.ReplaceAll(strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return -1
			}
			return r
		}, numbered), " ", ""), defs).Scan(&id)
	if err != nil {
		a.t.Fatal(err)
	}
	return id
}

func newID() string { return uuid.Must(uuid.NewV7()).String() }

func list(m map[string]any, key string) []map[string]any {
	raw, _ := m[key].([]any)
	out := make([]map[string]any, len(raw))
	for i, v := range raw {
		out[i], _ = v.(map[string]any)
	}
	return out
}

// TestStudySession is the M2 "done when": a full review session through the
// API (SYSTEM_DESIGN §14).
func TestStudySession(t *testing.T) {
	a := newTestAPI(t)
	const me = "owner@example.com"
	cidian := a.addEntry("词典", "ci2 dian3", "cídiǎn", "dictionary")
	a.addEntry("你好", "ni3 hao3", "nǐhǎo", "hello", "hi")

	// Account, created on first sight; the configured owner gets the owner role.
	if m := a.must(200, me, "GET", "/v1/me", nil); m["role"] != "owner" || m["timezone"] != "Asia/Jakarta" {
		t.Fatalf("me = %v", m)
	}
	device := newID()
	a.must(200, me, "PUT", "/v1/devices/"+device, map[string]any{"name": "Pixel"})

	// A deck allowing two new cards a day; creating it twice is a no-op.
	deck := newID()
	body := map[string]any{"id": deck, "name": "HSK 1", "settings": map[string]any{"new_per_day": 2}}
	a.must(200, me, "POST", "/v1/decks", body)
	if m := a.must(200, me, "POST", "/v1/decks", body); m["settings"].(map[string]any)["new_per_day"] != 2.0 {
		t.Fatalf("retried deck = %v", m)
	}

	// A dictionary note: its facts come from the dictionary, not the client (ADR-0009).
	note1 := newID()
	n := a.must(200, me, "POST", "/v1/decks/"+deck+"/notes", map[string]any{
		"id": note1, "entry_id": cidian, "fields": map[string]any{"meaning": "made up by a client", "notes": "lesson 1"},
	})
	f := n["fields"].(map[string]any)
	if f["hanzi"] != "词典" || f["pinyin"] != "cídiǎn" || f["meaning"] != "dictionary" || f["notes"] != "lesson 1" {
		t.Fatalf("note fields = %v", f)
	}
	if cards := list(n, "cards"); len(cards) != 2 || cards[0]["type"] != "recognition" || cards[1]["type"] != "recall" {
		t.Fatalf("cards = %v", cards)
	}
	firstCard := list(n, "cards")[0]["id"].(string)
	if want := study.CardID(uuid.MustParse(note1), "recognition").String(); firstCard != want {
		t.Fatalf("card id %s, want UUIDv5 %s", firstCard, want)
	}
	// AI- and dictionary-origin notes must name an entry.
	a.must(422, me, "POST", "/v1/decks/"+deck+"/notes", map[string]any{
		"id": newID(), "origin": "tutor", "fields": map[string]any{"hanzi": "猫", "meaning": "cat"},
	})
	a.must(200, me, "POST", "/v1/decks/"+deck+"/notes", map[string]any{
		"id": newID(), "fields": map[string]any{"hanzi": "猫", "pinyin": "māo", "meaning": "cat"},
	})

	// Queue: nothing due yet, two new cards (both of the oldest note).
	q := a.must(200, me, "GET", "/v1/study/queue", nil)
	if q["due_count"] != 0.0 || q["new_count"] != 2.0 || len(list(q, "new")) != 2 {
		t.Fatalf("queue = %v", q)
	}
	if list(q, "new")[0]["card_id"] != firstCard {
		t.Fatalf("first new card = %v, want %s", list(q, "new")[0], firstCard)
	}

	// Review it Good: it enters learning and is due again within minutes.
	review := map[string]any{
		"id": newID(), "card_id": firstCard, "rating": 3, "device_id": device,
		"reviewed_at": a.clock.Now().Format(time.RFC3339Nano), "duration_ms": 4200,
	}
	r := a.must(200, me, "POST", "/v1/reviews", review)
	card := r["card"].(map[string]any)
	if card["state"] != "learning" || r["duplicate"] != false {
		t.Fatalf("review = %v", r)
	}
	// The same review again (a network retry) changes nothing (PRD FR-R-3).
	if again := a.must(200, me, "POST", "/v1/reviews", review); again["duplicate"] != true ||
		again["card"].(map[string]any)["due_at"] != card["due_at"] {
		t.Fatalf("retry = %v", again)
	}
	var logs int
	_ = a.pool.QueryRow(context.Background(), `SELECT count(*) FROM review_logs`).Scan(&logs)
	if logs != 1 {
		t.Fatalf("review_logs rows = %d, want 1", logs)
	}

	// Now one card is due today and one new card is left.
	q = a.must(200, me, "GET", "/v1/study/queue", nil)
	if q["due_count"] != 1.0 || q["new_count"] != 1.0 {
		t.Fatalf("queue after a review = %v", q)
	}

	// Ten minutes later, Good again: it graduates and is due in days.
	a.clock.Advance(10 * time.Minute)
	review["id"], review["reviewed_at"] = newID(), a.clock.Now().Format(time.RFC3339Nano)
	card = a.must(200, me, "POST", "/v1/reviews", review)["card"].(map[string]any)
	if card["state"] != "review" {
		t.Fatalf("after second good: %v", card)
	}
	due, _ := time.Parse(time.RFC3339Nano, card["due_at"].(string))
	if due.Sub(a.clock.Now()) < 24*time.Hour {
		t.Fatalf("graduated card due %v, want days away", due)
	}

	// A late offline review (older than the last one) replays the whole log,
	// and the stored state equals a from-scratch replay.
	late := map[string]any{
		"id": newID(), "card_id": firstCard, "rating": 1, "device_id": device,
		"reviewed_at": a.clock.Now().Add(-5 * time.Minute).Format(time.RFC3339Nano),
	}
	if r := a.must(200, me, "POST", "/v1/reviews", late); r["replayed"] != true {
		t.Fatalf("late review = %v", r)
	}
	userID := uuid.MustParse(a.must(200, me, "GET", "/v1/me", nil)["id"].(string))
	if matched, err := a.study().RebuildCard(context.Background(), userID, uuid.MustParse(firstCard)); err != nil || !matched {
		t.Fatalf("rebuild matched=%v err=%v: stored state differs from replay", matched, err)
	}

	// Bad reviews.
	bad := func(field string, v any) map[string]any {
		b := map[string]any{"id": newID(), "card_id": firstCard, "rating": 3, "device_id": device,
			"reviewed_at": a.clock.Now().Format(time.RFC3339Nano)}
		b[field] = v
		return b
	}
	a.must(422, me, "POST", "/v1/reviews", bad("rating", 5))
	a.must(422, me, "POST", "/v1/reviews", bad("reviewed_at", a.clock.Now().Add(time.Hour).Format(time.RFC3339)))
	a.must(422, me, "POST", "/v1/reviews", bad("device_id", newID()))
	a.must(404, me, "POST", "/v1/reviews", bad("card_id", newID()))
	a.must(400, me, "POST", "/v1/reviews", `{"id": "x", "surprise": 1}`)

	// The next study day starts at 4 am Jakarta (21:00 UTC): a fresh allowance.
	q = a.must(200, me, "GET", "/v1/study/queue", nil)
	if q["new_count"] != 1.0 {
		t.Fatalf("new today = %v", q["new_count"])
	}
	a.clock.Set(time.Date(2026, 11, 14, 21, 0, 0, 0, time.UTC))
	q = a.must(200, me, "GET", "/v1/study/queue", nil)
	if q["new_count"] != 2.0 || q["day_start"] != "2026-11-14T21:00:00Z" {
		t.Fatalf("next day queue = %v", q)
	}

	// Suspending a card hides it; deleting a note removes its cards.
	recall := list(n, "cards")[1]["id"].(string)
	a.must(204, me, "PATCH", "/v1/cards/"+recall, map[string]any{"suspended": true})
	for _, c := range list(a.must(200, me, "GET", "/v1/study/queue", nil), "new") {
		if c["card_id"] == recall {
			t.Fatal("suspended card still queued")
		}
	}
	a.must(200, me, "PATCH", "/v1/notes/"+note1, map[string]any{"fields": map[string]any{"notes": "edited"}, "tags": []string{"hsk1"}})
	if m := a.must(200, me, "GET", "/v1/notes/"+note1, nil); m["fields"].(map[string]any)["notes"] != "edited" {
		t.Fatalf("edited note = %v", m)
	}
	a.must(204, me, "DELETE", "/v1/notes/"+note1, nil)
	a.must(404, me, "GET", "/v1/notes/"+note1, nil)
	a.must(404, me, "POST", "/v1/reviews", bad("id", newID()))
	q = a.must(200, me, "GET", "/v1/study/queue", nil)
	if q["due_count"] != 0.0 {
		t.Fatalf("deleted note's card still due: %v", q)
	}
	if logs := 0; a.pool.QueryRow(context.Background(), `SELECT count(*) FROM review_logs`).Scan(&logs) == nil && logs != 3 {
		t.Fatalf("review log has %d rows after delete; it is append-only", logs)
	}
}

func TestCSVImport(t *testing.T) {
	a := newTestAPI(t)
	const me = "andi@example.com"
	a.addEntry("词典", "ci2 dian3", "cídiǎn", "dictionary")
	deck := newID()
	a.must(200, me, "POST", "/v1/decks", map[string]any{"id": deck, "name": "Class"})

	csv := "汉字,pinyin,meaning\n" +
		"词典,,\n" + // filled from the dictionary
		"学习,xue2 xi2,to study\n" + // not in the test dictionary, but has a meaning
		"鑫,,\n" + // unknown and no meaning
		"学习,xuéxí,again\n" // duplicate within the file
	importID := newID()
	path := "/v1/decks/" + deck + "/import?import_id=" + importID
	job := a.must(202, me, "POST", path, csv)["job_id"].(float64)
	if again := a.must(202, me, "POST", path, csv)["job_id"].(float64); again != job {
		t.Fatalf("retried upload made job %v, want %v", again, job)
	}
	a.must(422, me, "POST", "/v1/decks/"+deck+"/import?import_id="+newID(), "\n\n")
	a.must(400, me, "POST", "/v1/decks/"+deck+"/import", csv)

	jobPath := fmt.Sprintf("/v1/jobs/%d", int64(job))
	if s := a.must(200, me, "GET", jobPath, nil); s["status"] != "queued" {
		t.Fatalf("job = %v", s)
	}
	w := newWorker(a.pool, a.clock, a.log)
	if ran, err := w.RunOnce(context.Background()); !ran || err != nil {
		t.Fatalf("worker ran=%v err=%v", ran, err)
	}
	s := a.must(200, me, "GET", jobPath, nil)
	res := s["result"].(map[string]any)
	if s["status"] != "succeeded" || res["imported"] != 2.0 || res["skipped"] != 1.0 || res["failed"] != 1.0 {
		t.Fatalf("job = %v", s)
	}
	if errs := list(res, "errors"); len(errs) != 1 || errs[0]["line"] != 4.0 {
		t.Fatalf("errors = %v", errs)
	}

	notes := list(a.must(200, me, "GET", "/v1/decks/"+deck+"/notes", nil), "notes")
	if len(notes) != 2 {
		t.Fatalf("notes = %v", notes)
	}
	// File order is kept, and the dictionary filled in 词典.
	f0, f1 := notes[0]["fields"].(map[string]any), notes[1]["fields"].(map[string]any)
	if f0["hanzi"] != "词典" || f0["pinyin"] != "cídiǎn" || notes[0]["entry_id"] == nil || notes[0]["origin"] != "csv" {
		t.Fatalf("first note = %v", notes[0])
	}
	if f1["hanzi"] != "学习" || f1["pinyin"] != "xuéxí" || notes[1]["entry_id"] != nil {
		t.Fatalf("second note = %v", notes[1])
	}

	// Running the same job again (a retry after a crash) adds nothing.
	var payload []byte
	var userID uuid.UUID
	if err := a.pool.QueryRow(context.Background(), `SELECT payload, user_id FROM jobs WHERE id = $1`, int64(job)).Scan(&payload, &userID); err != nil {
		t.Fatal(err)
	}
	out, err := a.study().ImportCSVJob(context.Background(), jobs.Job{ID: int64(job), Kind: study.JobImportCSV, UserID: userID, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if r := out.(study.ImportResult); r.Imported != 0 || r.Skipped != 3 {
		t.Fatalf("rerun = %+v, want nothing imported", r)
	}
	if notes := list(a.must(200, me, "GET", "/v1/decks/"+deck+"/notes", nil), "notes"); len(notes) != 2 {
		t.Fatalf("notes after rerun = %d", len(notes))
	}
}

// study returns a study service on the test database, for checks the API doesn't expose.
func (a *testAPI) study() *study.Service {
	return study.NewService(a.pool, a.clock, studyDictionary{dictionary.NewService(a.pool)},
		identity.NewService(a.pool, a.clock, ""), scheduling.NewStore(a.pool))
}

// publicRoutes need no sign-in: shared dictionary content.
var publicRoutes = map[string]bool{
	"GET /v1/dict/search":       true,
	"GET /v1/dict/entries/{id}": true,
}

// TestAuthorization calls every user-owned endpoint as a second user
// against the first user's data (SYSTEM_DESIGN §12.3). Nothing of the first
// user's may come back, and their data must be unchanged. A route without a
// case here fails the test.
func TestAuthorization(t *testing.T) {
	a := newTestAPI(t)
	const alice, bob = "alice@example.com", "bob@example.com"
	cidian := a.addEntry("词典", "ci2 dian3", "cídiǎn", "dictionary")
	mao := a.addEntry("猫", "mao1", "māo", "cat")

	// Alice's data.
	aliceMe := a.must(200, alice, "GET", "/v1/me", nil)
	aDevice, aDeck, aNote := newID(), newID(), newID()
	a.must(200, alice, "PUT", "/v1/devices/"+aDevice, map[string]any{"name": "Alice phone"})
	a.must(200, alice, "POST", "/v1/decks", map[string]any{"id": aDeck, "name": "Alice deck"})
	n := a.must(200, alice, "POST", "/v1/decks/"+aDeck+"/notes", map[string]any{
		"id": aNote, "fields": map[string]any{"hanzi": "秘密", "meaning": "alice secret"},
	})
	aCard := list(n, "cards")[0]["id"].(string)
	aReview := newID()
	a.must(200, alice, "POST", "/v1/reviews", map[string]any{
		"id": aReview, "card_id": aCard, "rating": 3, "device_id": aDevice, "reviewed_at": a.clock.Now().Format(time.RFC3339),
	})
	aJob := int64(a.must(202, alice, "POST", "/v1/decks/"+aDeck+"/import?import_id="+newID(), "你好,nǐhǎo,hello\n")["job_id"].(float64))
	// A dictionary note (cards never reviewed), and a word she marked known.
	aDictNote := newID()
	a.must(200, alice, "POST", "/v1/decks/"+aDeck+"/notes", map[string]any{"id": aDictNote, "entry_id": cidian, "fields": map[string]any{"notes": "alice secret"}})
	a.must(200, alice, "PUT", "/v1/knowledge/"+itoa(mao), map[string]any{"status": "known"})

	// Bob has his own device and deck, so failures can't be blamed on missing data.
	bDevice, bDeck := newID(), newID()
	a.must(200, bob, "PUT", "/v1/devices/"+bDevice, map[string]any{"name": "Bob phone"})
	a.must(200, bob, "POST", "/v1/decks", map[string]any{"id": bDeck, "name": "Bob deck"})
	bCard := list(a.must(200, bob, "POST", "/v1/decks/"+bDeck+"/notes", map[string]any{
		"id": newID(), "fields": map[string]any{"hanzi": "猫", "meaning": "cat"},
	}), "cards")[0]["id"].(string)

	// Strings that must never appear in a response to Bob.
	secrets := []string{alice, aliceMe["id"].(string), aDevice, aDeck, aNote, aDictNote, aCard, aReview, "alice secret", "Alice deck", "Alice phone"}

	type attempt struct {
		method, path string
		body         any
		status       int // expected status for Bob
	}
	cases := map[string][]attempt{
		"GET /v1/me":               {{"GET", "/v1/me", nil, 200}},
		"PATCH /v1/me/settings":    {{"PATCH", "/v1/me/settings", map[string]any{"tone_colours": true}, 200}},
		"GET /v1/devices":          {{"GET", "/v1/devices", nil, 200}},
		"PUT /v1/devices/{id}":     {{"PUT", "/v1/devices/" + aDevice, map[string]any{"name": "stolen"}, 404}},
		"DELETE /v1/devices/{id}":  {{"DELETE", "/v1/devices/" + aDevice, nil, 404}},
		"GET /v1/decks":            {{"GET", "/v1/decks", nil, 200}},
		"POST /v1/decks":           {{"POST", "/v1/decks", map[string]any{"id": aDeck, "name": "mine now"}, 409}},
		"GET /v1/decks/{id}":       {{"GET", "/v1/decks/" + aDeck, nil, 404}},
		"PATCH /v1/decks/{id}":     {{"PATCH", "/v1/decks/" + aDeck, map[string]any{"name": "renamed"}, 404}},
		"DELETE /v1/decks/{id}":    {{"DELETE", "/v1/decks/" + aDeck, nil, 404}},
		"GET /v1/decks/{id}/notes": {{"GET", "/v1/decks/" + aDeck + "/notes", nil, 404}},
		"POST /v1/decks/{id}/notes": {
			{"POST", "/v1/decks/" + aDeck + "/notes", map[string]any{"id": newID(), "fields": map[string]any{"hanzi": "x", "meaning": "y"}}, 404},
			// Bob's deck, Alice's note id: an id clash, not a way to read her note.
			{"POST", "/v1/decks/" + bDeck + "/notes", map[string]any{"id": aNote, "fields": map[string]any{"hanzi": "x", "meaning": "y"}}, 409},
		},
		"POST /v1/decks/{id}/import": {{"POST", "/v1/decks/" + aDeck + "/import?import_id=" + newID(), "猫,māo,cat\n", 404}},
		"GET /v1/notes":              {{"GET", "/v1/notes?entry_id=" + itoa(cidian), nil, 200}},
		"GET /v1/notes/{id}":         {{"GET", "/v1/notes/" + aNote, nil, 404}},
		"PATCH /v1/notes/{id}":       {{"PATCH", "/v1/notes/" + aNote, map[string]any{"fields": map[string]any{"meaning": "hacked"}}, 404}},
		"DELETE /v1/notes/{id}":      {{"DELETE", "/v1/notes/" + aNote, nil, 404}},
		"PATCH /v1/cards/{id}":       {{"PATCH", "/v1/cards/" + aCard, map[string]any{"suspended": true}, 404}},
		"GET /v1/study/queue": {
			{"GET", "/v1/study/queue", nil, 200},
			{"GET", "/v1/study/queue?deck=" + aDeck, nil, 404},
		},
		"POST /v1/reviews": {
			{"POST", "/v1/reviews", map[string]any{"id": newID(), "card_id": aCard, "rating": 1, "device_id": bDevice, "reviewed_at": a.clock.Now().Format(time.RFC3339)}, 404},
			// Alice's device on Bob's card.
			{"POST", "/v1/reviews", map[string]any{"id": newID(), "card_id": bCard, "rating": 1, "device_id": aDevice, "reviewed_at": a.clock.Now().Format(time.RFC3339)}, 422},
			// Reusing Alice's review id on Bob's own card is a clash.
			{"POST", "/v1/reviews", map[string]any{"id": aReview, "card_id": bCard, "rating": 1, "device_id": bDevice, "reviewed_at": a.clock.Now().Format(time.RFC3339)}, 409},
		},
		"GET /v1/study/summary": {{"GET", "/v1/study/summary", nil, 200}},
		"GET /v1/jobs/{id}":     {{"GET", fmt.Sprintf("/v1/jobs/%d", aJob), nil, 404}},
		"GET /v1/knowledge":     {{"GET", "/v1/knowledge?entry_ids=" + itoa(cidian) + "," + itoa(mao), nil, 200}},
		// Bob marking a word only writes his own row: Alice's cards for it stay as they were.
		"PUT /v1/knowledge/{id}": {{"PUT", "/v1/knowledge/" + itoa(cidian), map[string]any{"status": "known"}, 200}},
		// Alice knows 猫; Bob doesn't, so there is nothing of his to remove.
		"DELETE /v1/knowledge/{id}": {{"DELETE", "/v1/knowledge/" + itoa(mao), nil, 404}},
	}

	// Route registry check: every user-owned route has a case.
	err := chi.Walk(a.router.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		if !strings.HasPrefix(route, "/v1/") || publicRoutes[key] {
			return nil
		}
		if _, ok := cases[key]; !ok {
			t.Errorf("route %s has no cross-user authorization case", key)
		}
		// Every user-owned route needs a sign-in.
		path := strings.ReplaceAll(route, "{id}", newID())
		if r := a.call("", method, path, nil); r.status != http.StatusUnauthorized {
			t.Errorf("%s without sign-in = %d, want 401", key, r.status)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for key, attempts := range cases {
		for _, at := range attempts {
			r := a.call(bob, at.method, at.path, at.body)
			if r.status != at.status {
				t.Errorf("%s as Bob (%s) = %d, want %d: %s", key, at.path, r.status, at.status, r.body)
			}
			for _, s := range secrets {
				if bytes.Contains(r.body, []byte(s)) {
					t.Errorf("%s as Bob leaked %q: %s", key, s, r.body)
				}
			}
		}
	}

	// Alice's data is untouched.
	if d := a.must(200, alice, "GET", "/v1/decks/"+aDeck, nil); d["name"] != "Alice deck" {
		t.Errorf("Alice's deck = %v", d)
	}
	nn := a.must(200, alice, "GET", "/v1/notes/"+aNote, nil)
	if nn["fields"].(map[string]any)["meaning"] != "alice secret" || list(nn, "cards")[0]["suspended"] != false {
		t.Errorf("Alice's note = %v", nn)
	}
	if ds := list(a.must(200, alice, "GET", "/v1/devices", nil), "devices"); len(ds) != 1 || ds[0]["name"] != "Alice phone" {
		t.Errorf("Alice's devices = %v", ds)
	}
	if m := a.must(200, alice, "GET", "/v1/me", nil); m["settings"].(map[string]any)["tone_colours"] != false {
		t.Errorf("Alice's settings changed: %v", m)
	}
	var logs int
	_ = a.pool.QueryRow(context.Background(), `SELECT count(*) FROM review_logs`).Scan(&logs)
	if logs != 1 {
		t.Errorf("review_logs = %d, want only Alice's 1", logs)
	}
	for _, c := range list(a.must(200, alice, "GET", "/v1/notes/"+aDictNote, nil), "cards") {
		if c["suspended"] != false {
			t.Errorf("Bob's \"I already know this\" suspended Alice's card: %v", c)
		}
	}
	if ks := list(a.must(200, alice, "GET", "/v1/knowledge?entry_ids="+itoa(cidian)+","+itoa(mao), nil), "knowledge"); len(ks) != 1 || ks[0]["entry_id"] != float64(mao) {
		t.Errorf("Alice's knowledge = %v", ks)
	}
}
