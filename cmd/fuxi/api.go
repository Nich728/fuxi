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
	"github.com/Nich728/fuxi/internal/identity"
	"github.com/Nich728/fuxi/internal/jobs"
	"github.com/Nich728/fuxi/internal/platform/clock"
	"github.com/Nich728/fuxi/internal/platform/config"
	"github.com/Nich728/fuxi/internal/platform/db"
	"github.com/Nich728/fuxi/internal/platform/httpx"
	"github.com/Nich728/fuxi/internal/scheduling"
	"github.com/Nich728/fuxi/internal/study"
)

func runAPI(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := cfg.Auth.Validate(); err != nil {
		return err
	}
	clk := clock.System{}
	auth, err := newAuthenticator(cfg.Auth, clk, log)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           newRouter(pool, log, auth, clk, cfg.Auth.OwnerEmail),
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

// newAuthenticator returns the sign-in check for the configured mode (ADR-0018).
func newAuthenticator(a config.Auth, clk clock.Clock, log *slog.Logger) (identity.Authenticator, error) {
	if a.Mode == config.AuthDev {
		log.Warn("FUXI_AUTH=dev: every request is signed in without a password. Never use this outside local development.",
			"email", a.DevEmail)
		return identity.Dev{DefaultEmail: a.DevEmail}, nil
	}
	return identity.NewAccess(a.AccessTeamDomain, a.AccessAudience, clk, nil)
}

// newRouter wires every module's routes (SYSTEM_DESIGN §10).
func newRouter(pool *pgxpool.Pool, log *slog.Logger, auth identity.Authenticator, clk clock.Clock, ownerEmail string) http.Handler {
	dictSvc := dictionary.NewService(pool)
	dict := dictionary.NewHandler(dictSvc, log.With("module", "dictionary"))
	ids := identity.NewService(pool, clk, ownerEmail)
	studySvc := study.NewService(pool, clk, studyDictionary{dictSvc}, ids, scheduling.NewStore(pool))

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// No RealIP: it trusts client-supplied X-Forwarded-For. Add a proxy-aware
	// equivalent when the deploy target's proxy headers are known (M3).
	r.Use(requestLogger(log))
	r.Use(middleware.Recoverer)

	r.Get("/healthz", httpx.Healthz)
	r.Get("/readyz", httpx.Readyz(pool))

	r.Route("/v1", func(r chi.Router) {
		// Dictionary content is shared and the same for everyone.
		dict.Routes(r)

		// Everything else belongs to the signed-in user.
		r.Group(func(r chi.Router) {
			r.Use(identity.Middleware(auth, ids, log.With("module", "identity")))
			identity.NewHandler(ids, log.With("module", "identity")).Routes(r)
			study.NewHandler(studySvc, log.With("module", "study")).Routes(r)
			jobs.NewHTTPHandler(pool, log.With("module", "jobs")).Routes(r)
		})
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
