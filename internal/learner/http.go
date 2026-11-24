package learner

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Nich728/fuxi/internal/platform/httpx"
	"github.com/Nich728/fuxi/internal/platform/principal"
)

// Handler serves the word knowledge endpoints (SYSTEM_DESIGN §10).
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the endpoints. They must sit behind the identity middleware.
// {id} is a dictionary entry id.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/knowledge", h.list)
	r.Put("/knowledge/{id}", h.markKnown)
	r.Delete("/knowledge/{id}", h.unmark)
}

// list: GET /knowledge?entry_ids=1,2,3 → {"knowledge": [...]} for the entries that have a row.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	var ids []int64
	if v := r.URL.Query().Get("entry_ids"); v != "" {
		for _, part := range strings.Split(v, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil || id < 1 {
				httpx.WriteProblem(w, http.StatusBadRequest, "entry_ids must be a comma-separated list of entry ids")
				return
			}
			ids = append(ids, id)
		}
	}
	ks, err := h.svc.Knowledge(r.Context(), p.UserID, ids)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"knowledge": ks})
}

// markKnown: PUT /knowledge/{entry id} {"status": "known"} → the row and how many cards were suspended.
func (h *Handler) markKnown(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := entryParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.Status != "known" {
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, `status must be "known"`)
		return
	}
	k, suspended, err := h.svc.MarkKnown(r.Context(), p.UserID, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"knowledge": k, "cards_suspended": suspended})
}

// unmark: DELETE /knowledge/{entry id} → how many cards were brought back.
func (h *Handler) unmark(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := entryParam(w, r)
	if !ok {
		return
	}
	restored, err := h.svc.Unmark(r.Context(), p.UserID, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cards_restored": restored})
}

func entryParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		httpx.WriteProblem(w, http.StatusNotFound, "not found")
		return 0, false
	}
	return id, true
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteProblem(w, http.StatusNotFound, "not found")
	case errors.Is(err, ErrInvalid):
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, err.Error())
	default:
		h.log.Error("learner request failed", "route", chi.RouteContext(r.Context()).RoutePattern(), "err", err)
		httpx.WriteProblem(w, http.StatusInternalServerError, "")
	}
}
