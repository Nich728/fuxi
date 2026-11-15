package identity

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Nich728/fuxi/internal/platform/httpx"
	"github.com/Nich728/fuxi/internal/platform/principal"
)

// Middleware authenticates every request and puts the principal in its
// context. Requests without a valid sign-in get 401.
func Middleware(auth Authenticator, svc *Service, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email, err := auth.Authenticate(r)
			if errors.Is(err, ErrUnauthenticated) {
				log.Debug("unauthenticated request", "route", r.URL.Path, "err", err)
				httpx.WriteProblem(w, http.StatusUnauthorized, "sign in required")
				return
			}
			if err != nil {
				log.Error("authenticate", "err", err)
				httpx.WriteProblem(w, http.StatusServiceUnavailable, "sign-in check unavailable")
				return
			}
			p, err := svc.Resolve(r.Context(), email)
			if errors.Is(err, ErrInvalid) {
				httpx.WriteProblem(w, http.StatusUnauthorized, "sign in required")
				return
			}
			if err != nil {
				log.Error("resolve user", "err", err)
				httpx.WriteProblem(w, http.StatusInternalServerError, "")
				return
			}
			next.ServeHTTP(w, r.WithContext(principal.With(r.Context(), p)))
		})
	}
}

// Handler serves the account and device endpoints (SYSTEM_DESIGN §10).
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the endpoints. They must sit behind Middleware.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/me", h.me)
	r.Patch("/me/settings", h.updateSettings)
	r.Get("/devices", h.devices)
	r.Put("/devices/{id}", h.registerDevice)
	r.Delete("/devices/{id}", h.removeDevice)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	u, err := h.svc.Me(r.Context(), p.UserID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	var body SettingsPatch
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	u, err := h.svc.UpdateSettings(r.Context(), p.UserID, body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) devices(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	ds, err := h.svc.Devices(r.Context(), p.UserID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"devices": ds})
}

// registerDevice: PUT /devices/{id} {"name": "Pixel 8"}. Idempotent.
func (h *Handler) registerDevice(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	d, err := h.svc.RegisterDevice(r.Context(), p.UserID, id, body.Name)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

func (h *Handler) removeDevice(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.RemoveDevice(r.Context(), p.UserID, id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteProblem(w, http.StatusNotFound, "not found")
	case errors.Is(err, ErrInvalid):
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, err.Error())
	default:
		h.log.Error("identity request failed", "route", r.URL.Path, "err", err)
		httpx.WriteProblem(w, http.StatusInternalServerError, "")
	}
}
