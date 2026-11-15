package jobs

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Nich728/fuxi/internal/platform/httpx"
	"github.com/Nich728/fuxi/internal/platform/principal"
)

// HTTPHandler serves GET /jobs/{id}, so a client can follow an import it started.
type HTTPHandler struct {
	db  DBTX
	log *slog.Logger
}

// NewHTTPHandler returns the job status endpoint.
func NewHTTPHandler(db DBTX, log *slog.Logger) *HTTPHandler {
	return &HTTPHandler{db: db, log: log}
}

// Routes mounts GET /jobs/{id}. It must sit behind the identity middleware.
func (h *HTTPHandler) Routes(r chi.Router) {
	r.Get("/jobs/{id}", h.get)
}

func (h *HTTPHandler) get(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteProblem(w, http.StatusNotFound, "not found")
		return
	}
	s, err := Get(r.Context(), h.db, p.UserID, id)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteProblem(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		h.log.Error("get job", "err", err)
		httpx.WriteProblem(w, http.StatusInternalServerError, "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}
