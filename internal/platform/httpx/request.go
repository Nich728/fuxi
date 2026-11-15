package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MaxJSONBody bounds JSON request bodies. Larger uploads (CSV) set their own limit.
const MaxJSONBody = 64 << 10

// DecodeJSON reads one JSON object from the body into v. Unknown fields are
// rejected so typos in a client fail loudly. On error it has already written
// a 400 or 413 problem and returns false.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxJSONBody))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil && dec.Decode(&struct{}{}) != io.EOF {
		err = errors.New("body must contain a single JSON object")
	}
	if err == nil {
		return true
	}
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		WriteProblem(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("body larger than %d bytes", tooBig.Limit))
		return false
	}
	WriteProblem(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
	return false
}

// UUIDParam parses a UUID path parameter. A malformed id can't name anything
// the user owns, so it is reported as 404 like any other unknown id.
func UUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		WriteProblem(w, http.StatusNotFound, "not found")
		return uuid.UUID{}, false
	}
	return id, true
}
