package dictionary

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Nich728/fuxi/internal/platform/httpx"
)

// Handler serves the dictionary endpoints (SYSTEM_DESIGN §10, "Dictionary").
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts GET /dict/search and GET /dict/entries/{id}.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/dict/search", h.search)
	r.Get("/dict/entries/{id}", h.entry)
}

// search: GET /dict/search?q=ma&limit=20 → {"results": [...], "segmented": false}
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	limit, err := optionalInt(r, "limit", 1, MaxLimit)
	if err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.svc.Search(r.Context(), r.URL.Query().Get("q"), limit)
	if errors.Is(err, ErrBadQuery) {
		httpx.WriteProblem(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// entry: GET /dict/entries/123?max_hsk=3 → the entry with example sentences.
func (h *Handler) entry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteProblem(w, http.StatusNotFound, "no such entry")
		return
	}
	maxHSK, err := optionalInt(r, "max_hsk", 1, 7)
	if err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, err.Error())
		return
	}
	d, err := h.svc.Get(r.Context(), id, maxHSK)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteProblem(w, http.StatusNotFound, "no such entry")
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Error("dictionary request failed", "route", r.URL.Path, "err", err)
	httpx.WriteProblem(w, http.StatusInternalServerError, "")
}

type paramError struct{ name string }

func (e paramError) Error() string { return "invalid " + e.name }

// optionalInt reads an integer query parameter in [lo, hi]; absent means 0.
func optionalInt(r *http.Request, name string, lo, hi int) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		return 0, paramError{name}
	}
	return n, nil
}
