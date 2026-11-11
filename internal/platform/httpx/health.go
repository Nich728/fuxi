// Package httpx holds HTTP helpers shared by every module.
package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Pinger is anything that can check a dependency, such as *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Healthz reports that the process is up. It never touches dependencies,
// so a slow database can't make the orchestrator restart a healthy process.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz reports whether the process can serve traffic: the database must answer.
func Readyz(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status":   "unavailable",
				"database": "unreachable",
			})
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
	}
}

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
