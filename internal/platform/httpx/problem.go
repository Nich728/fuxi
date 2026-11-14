package httpx

import (
	"encoding/json"
	"net/http"
)

// Problem is an RFC 9457 problem details body (SYSTEM_DESIGN §10).
type Problem struct {
	Type   string `json:"type,omitempty"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// WriteProblem writes an application/problem+json response.
// Title is the generic status text; detail explains this occurrence.
func WriteProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	})
}
