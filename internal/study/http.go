package study

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Nich728/fuxi/internal/platform/httpx"
	"github.com/Nich728/fuxi/internal/platform/principal"
)

// Handler serves the deck, note, card, import, queue and review endpoints
// (SYSTEM_DESIGN §10).
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the endpoints. They must sit behind the identity middleware.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/decks", h.listDecks)
	r.Post("/decks", h.createDeck)
	r.Get("/decks/{id}", h.getDeck)
	r.Patch("/decks/{id}", h.updateDeck)
	r.Delete("/decks/{id}", h.deleteDeck)
	r.Get("/decks/{id}/notes", h.listNotes)
	r.Post("/decks/{id}/notes", h.createNote)
	r.Post("/decks/{id}/import", h.importCSV)
	r.Get("/notes/{id}", h.getNote)
	r.Patch("/notes/{id}", h.updateNote)
	r.Delete("/notes/{id}", h.deleteNote)
	r.Patch("/cards/{id}", h.updateCard)
	r.Get("/study/queue", h.queue)
	r.Post("/reviews", h.submitReview)
}

func (h *Handler) listDecks(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	decks, err := h.svc.Decks(r.Context(), p.UserID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"decks": decks})
}

// createDeck: POST /decks {"id": "<uuidv7>", "name": "HSK 3", "settings": {...}}
func (h *Handler) createDeck(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	var body struct {
		ID       uuid.UUID          `json:"id"`
		Name     string             `json:"name"`
		Settings *DeckSettingsPatch `json:"settings"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.ID == uuid.Nil {
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, "id is required")
		return
	}
	d, err := h.svc.CreateDeck(r.Context(), p.UserID, body.ID, body.Name, body.Settings)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

func (h *Handler) getDeck(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	d, err := h.svc.Deck(r.Context(), p.UserID, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

func (h *Handler) updateDeck(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Name     *string            `json:"name"`
		Settings *DeckSettingsPatch `json:"settings"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	d, err := h.svc.UpdateDeck(r.Context(), p.UserID, id, body.Name, body.Settings)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

func (h *Handler) deleteDeck(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteDeck(r.Context(), p.UserID, id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listNotes: GET /decks/{id}/notes?after=<note id>&limit=100
// The response's next_cursor is set when there may be more.
func (h *Handler) listNotes(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	var after uuid.UUID
	if v := r.URL.Query().Get("after"); v != "" {
		var err error
		if after, err = uuid.Parse(v); err != nil {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid after")
			return
		}
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > MaxPage {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	notes, err := h.svc.Notes(r.Context(), p.UserID, id, after, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var next *uuid.UUID
	if len(notes) == limit {
		next = &notes[len(notes)-1].ID
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"notes": notes, "next_cursor": next})
}

// createNote: POST /decks/{id}/notes {"id": "...", "entry_id": 123}
func (h *Handler) createNote(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	deckID, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body NewNote
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	n, err := h.svc.CreateNote(r.Context(), p.UserID, deckID, body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, n)
}

// importCSV: POST /decks/{id}/import?import_id=<uuid>, body = the CSV file.
// Returns 202 {"job_id": 42}; follow it with GET /jobs/42.
func (h *Handler) importCSV(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	deckID, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	importID, err := uuid.Parse(r.URL.Query().Get("import_id"))
	if err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, "import_id query parameter (a UUID) is required")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxCSVBytes))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		httpx.WriteProblem(w, http.StatusRequestEntityTooLarge, "CSV files are limited to 2 MB")
		return
	}
	if err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, "could not read body")
		return
	}
	jobID, err := h.svc.StartImport(r.Context(), p.UserID, deckID, importID, data)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]int64{"job_id": jobID})
}

func (h *Handler) getNote(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := h.svc.Note(r.Context(), p.UserID, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, n)
}

func (h *Handler) updateNote(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body NotePatch
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	n, err := h.svc.UpdateNote(r.Context(), p.UserID, id, body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, n)
}

func (h *Handler) deleteNote(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteNote(r.Context(), p.UserID, id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateCard: PATCH /cards/{id} {"suspended": true}
func (h *Handler) updateCard(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, ok := httpx.UUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Suspended *bool `json:"suspended"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.Suspended == nil {
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, "suspended is required")
		return
	}
	if err := h.svc.SetSuspended(r.Context(), p.UserID, id, *body.Suspended); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// queue: GET /study/queue?deck=<id>&limit=100
func (h *Handler) queue(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	var deck *uuid.UUID
	if v := r.URL.Query().Get("deck"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteProblem(w, http.StatusNotFound, "not found")
			return
		}
		deck = &id
	}
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > MaxQueue {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	q, err := h.svc.Queue(r.Context(), p, deck, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, q)
}

// submitReview: POST /reviews {"id", "card_id", "rating", "reviewed_at", "duration_ms", "device_id"}
func (h *Handler) submitReview(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	var body NewReview
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.SubmitReview(r.Context(), p.UserID, body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteProblem(w, http.StatusNotFound, "not found")
	case errors.Is(err, ErrInvalid):
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrConflict):
		httpx.WriteProblem(w, http.StatusConflict, err.Error())
	default:
		h.log.Error("study request failed", "route", chi.RouteContext(r.Context()).RoutePattern(), "err", err)
		httpx.WriteProblem(w, http.StatusInternalServerError, "")
	}
}
