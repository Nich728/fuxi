package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWebApp(t *testing.T) {
	files := fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>Fuxi</title>")},
		"sw.js":                {Data: []byte("self.addEventListener('fetch', () => {})")},
		"manifest.webmanifest": {Data: []byte(`{"name": "Fuxi"}`)},
		"assets/app-1a2b3c.js": {Data: []byte("console.log('app')")},
	}
	h := WebApp(files)

	tests := []struct {
		name, method, path string
		status             int
		body, cache        string
	}{
		{"root", "GET", "/", 200, "<title>Fuxi</title>", "no-cache"},
		{"client route", "GET", "/decks/0193a000-0000-7000-8000-00000000d001", 200, "<title>Fuxi</title>", "no-cache"},
		{"hashed asset", "GET", "/assets/app-1a2b3c.js", 200, "console.log", "public, max-age=31536000, immutable"},
		{"service worker", "GET", "/sw.js", 200, "addEventListener", "no-cache"},
		{"manifest", "GET", "/manifest.webmanifest", 200, "Fuxi", "no-cache"},
		{"missing asset", "GET", "/assets/gone-000000.js", 404, "", ""},
		{"path escape stays inside", "GET", "/../../etc/passwd", 200, "<title>Fuxi</title>", "no-cache"},
		{"head", "HEAD", "/", 200, "", "no-cache"},
		{"post", "POST", "/", 405, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.status, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.body) {
				t.Errorf("body = %q, want it to contain %q", rec.Body.String(), tt.body)
			}
			if got := rec.Header().Get("Cache-Control"); tt.cache != "" && got != tt.cache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cache)
			}
		})
	}
}

func TestWebAppIndexNotRedirected(t *testing.T) {
	h := WebApp(fstest.MapFS{"index.html": {Data: []byte("app")}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/study", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "app" {
		t.Fatalf("got %d %q, want 200 \"app\"", rec.Code, rec.Body.String())
	}
}
