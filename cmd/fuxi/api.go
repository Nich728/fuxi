package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nich728/fuxi/internal/dictionary"
	"github.com/Nich728/fuxi/internal/platform/config"
	"github.com/Nich728/fuxi/internal/platform/db"
	"github.com/Nich728/fuxi/internal/platform/httpx"
)

func runAPI(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           newRouter(pool, log),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newRouter(pool *pgxpool.Pool, log *slog.Logger) http.Handler {
	dict := dictionary.NewHandler(dictionary.NewService(pool), log.With("module", "dictionary"))

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// No RealIP: it trusts client-supplied X-Forwarded-For. Add a proxy-aware
	// equivalent when the deploy target's proxy headers are known (M3).
	r.Use(requestLogger(log))
	r.Use(middleware.Recoverer)

	r.Get("/healthz", httpx.Healthz)
	r.Get("/readyz", httpx.Readyz(pool))

	r.Route("/v1", func(r chi.Router) {
		// Module routes (SYSTEM_DESIGN §10). Dictionary content is shared and
		// public; user-owned routes arrive with auth in M2.
		dict.Routes(r)
	})
	return r
}

// requestLogger logs one line per request with the fields from SYSTEM_DESIGN §12.2.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			// Health probes are frequent and uninteresting.
			if route == "/healthz" || route == "/readyz" {
				return
			}
			log.Info("request",
				"request_id", middleware.GetReqID(r.Context()),
				"method", r.Method,
				"route", route,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
