package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Nich728/fuxi/internal/identity"
)

// Endpoints added for the web client (M3).

// TestKnownWord is "I already know this" (PRD V7, SYSTEM_DESIGN §9.7).
func TestKnownWord(t *testing.T) {
	a := newTestAPI(t)
	const me = "me@example.com"
	cidian := a.addEntry("词典", "ci2 dian3", "cídiǎn", "dictionary")
	device, deck := newID(), newID()
	a.must(200, me, "PUT", "/v1/devices/"+device, map[string]any{"name": "phone"})
	a.must(200, me, "POST", "/v1/decks", map[string]any{"id": deck, "name": "HSK 1"})
	n := a.must(200, me, "POST", "/v1/decks/"+deck+"/notes", map[string]any{"id": newID(), "entry_id": cidian})
	recognition := list(n, "cards")[0]["id"].(string)
	path := "/v1/knowledge/" + itoa(cidian)

	// The word page can find the note by its entry.
	if notes := list(a.must(200, me, "GET", "/v1/notes?entry_id="+itoa(cidian), nil), "notes"); len(notes) != 1 || notes[0]["deck_id"] != deck {
		t.Fatalf("notes for entry = %v", notes)
	}
	a.must(400, me, "GET", "/v1/notes", nil)

	// Review the recognition card once; only the never-reviewed recall card is suspended.
	a.must(200, me, "POST", "/v1/reviews", map[string]any{
		"id": newID(), "card_id": recognition, "rating": 3, "device_id": device, "reviewed_at": a.clock.Now().Format(time.RFC3339),
	})
	r := a.must(200, me, "PUT", path, map[string]any{"status": "known"})
	k := r["knowledge"].(map[string]any)
	if r["cards_suspended"] != 1.0 || k["status"] != "known" || k["source"] != "manual" || k["entry_id"] != float64(cidian) {
		t.Fatalf("mark known = %v", r)
	}
	if q := a.must(200, me, "GET", "/v1/study/queue", nil); q["new_count"] != 0.0 {
		t.Fatalf("queue still offers the known word's new card: %v", q)
	}
	// Repeating it is a no-op.
	if r := a.must(200, me, "PUT", path, map[string]any{"status": "known"}); r["cards_suspended"] != 0.0 {
		t.Fatalf("second mark = %v", r)
	}
	ks := list(a.must(200, me, "GET", "/v1/knowledge?entry_ids="+itoa(cidian)+",999999", nil), "knowledge")
	if len(ks) != 1 || ks[0]["status"] != "known" {
		t.Fatalf("knowledge = %v", ks)
	}

	// Undo brings the recall card back.
	if r := a.must(200, me, "DELETE", path, nil); r["cards_restored"] != 1.0 {
		t.Fatalf("unmark = %v", r)
	}
	if q := a.must(200, me, "GET", "/v1/study/queue", nil); q["new_count"] != 1.0 {
		t.Fatalf("queue after undo = %v", q)
	}
	a.must(404, me, "DELETE", path, nil)

	// Bad input.
	a.must(404, me, "PUT", "/v1/knowledge/999999", map[string]any{"status": "known"}) // no such entry
	a.must(422, me, "PUT", path, map[string]any{"status": "learning"})
	a.must(404, me, "PUT", "/v1/knowledge/abc", map[string]any{"status": "known"})
	a.must(400, me, "GET", "/v1/knowledge?entry_ids=1,x", nil)
	a.must(422, me, "GET", "/v1/knowledge?entry_ids="+strings.Repeat("1,", 200)+"1", nil)
	if ks := list(a.must(200, me, "GET", "/v1/knowledge", nil), "knowledge"); len(ks) != 0 {
		t.Fatalf("knowledge with no ids = %v", ks)
	}
}

// TestQueuePreviewAndSummary covers the rating intervals on the review
// screen and the home screen's counts and seal week.
func TestQueuePreviewAndSummary(t *testing.T) {
	a := newTestAPI(t) // 09:00 on 14 November in Jakarta
	const me = "me@example.com"
	device, deck := newID(), newID()
	a.must(200, me, "PUT", "/v1/devices/"+device, map[string]any{"name": "phone"})
	a.must(200, me, "POST", "/v1/decks", map[string]any{"id": deck, "name": "HSK 1", "settings": map[string]any{"new_per_day": 3}})
	for _, h := range []string{"一", "二"} {
		a.must(200, me, "POST", "/v1/decks/"+deck+"/notes", map[string]any{"id": newID(), "fields": map[string]any{"hanzi": h, "meaning": "number"}})
	}

	// Previews: each rating is due no earlier than the one before.
	q := a.must(200, me, "GET", "/v1/study/queue", nil)
	first := list(q, "new")[0]
	pv := first["preview"].(map[string]any)
	ts := func(k string) time.Time {
		v, err := time.Parse(time.RFC3339Nano, pv[k].(string))
		if err != nil {
			t.Fatalf("preview %s: %v", k, err)
		}
		return v
	}
	if !ts("again").Before(ts("good")) || ts("hard").After(ts("good")) || !ts("good").Before(ts("easy")) {
		t.Fatalf("preview not ordered: %v", pv)
	}
	// Rating Good now gives exactly the previewed time.
	r := a.must(200, me, "POST", "/v1/reviews", map[string]any{
		"id": newID(), "card_id": first["card_id"], "rating": 3, "device_id": device, "reviewed_at": a.clock.Now().Format(time.RFC3339Nano),
	})
	if due, _ := time.Parse(time.RFC3339Nano, r["card"].(map[string]any)["due_at"].(string)); !due.Equal(ts("good")) {
		t.Fatalf("good due %v, preview said %v", due, ts("good"))
	}

	// Summary: 1 due (the learning card), 2 of 3 new still allowed; today has a review.
	s := a.must(200, me, "GET", "/v1/study/summary", nil)
	decks := list(s, "decks")
	if s["today"] != "2026-11-14" || s["due_count"] != 1.0 || s["new_count"] != 2.0 ||
		len(decks) != 1 || decks[0]["deck_id"] != deck || decks[0]["due"] != 1.0 || decks[0]["new"] != 2.0 {
		t.Fatalf("summary = %v", s)
	}
	days := list(s, "days")
	if len(days) != 7 || days[0]["date"] != "2026-11-08" || days[6]["date"] != "2026-11-14" || days[6]["reviews"] != 1.0 || days[5]["reviews"] != 0.0 {
		t.Fatalf("days = %v", days)
	}

	// 01:00 the next night in Jakarta is still the 14th's study day (day starts at 04:00).
	a.clock.Advance(16 * time.Hour)
	a.must(200, me, "POST", "/v1/reviews", map[string]any{
		"id": newID(), "card_id": first["card_id"], "rating": 3, "device_id": device, "reviewed_at": a.clock.Now().Format(time.RFC3339Nano),
	})
	s = a.must(200, me, "GET", "/v1/study/summary", nil)
	if days := list(s, "days"); s["today"] != "2026-11-14" || days[6]["reviews"] != 2.0 {
		t.Fatalf("after a late-night review: today %v, days %v", s["today"], days)
	}
	// After 04:00 it's the 15th; yesterday keeps its seal.
	a.clock.Advance(4 * time.Hour)
	s = a.must(200, me, "GET", "/v1/study/summary", nil)
	if days := list(s, "days"); s["today"] != "2026-11-15" || days[5]["reviews"] != 2.0 || days[6]["reviews"] != 0.0 {
		t.Fatalf("next day: today %v, days %v", s["today"], days)
	}
}

// TestWebAppRoute checks the PWA is served next to the API without
// shadowing it.
func TestWebAppRoute(t *testing.T) {
	a := newTestAPI(t)
	web := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>Fuxi</title>")}}
	router := newRouter(routerDeps{pool: a.pool, log: a.log, auth: identity.Dev{}, clock: a.clock, web: web})

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	for _, p := range []string{"/", "/review", "/decks/0193a000-0000-7000-8000-00000000d001"} {
		if rec := get(p); rec.Code != 200 || !strings.Contains(rec.Body.String(), "<title>Fuxi</title>") {
			t.Errorf("GET %s = %d %q, want the app", p, rec.Code, rec.Body.String())
		}
	}
	// API paths never fall through to the app.
	if rec := get("/v1/no-such-route"); rec.Code != 404 || strings.Contains(rec.Body.String(), "<title>") {
		t.Errorf("GET /v1/no-such-route = %d %q", rec.Code, rec.Body.String())
	}
	if rec := get("/healthz"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "ok") {
		t.Errorf("GET /healthz = %d %q", rec.Code, rec.Body.String())
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
