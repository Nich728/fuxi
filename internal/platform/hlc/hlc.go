// Package hlc issues hybrid logical clock values for edits made through the
// API (SYSTEM_DESIGN §7.3).
package hlc

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Nich728/fuxi/internal/platform/clock"
)

// Clock issues values of the form {wall_ms:013d}:{counter:05d}:{node}. Until
// offline sync (M5) every edit comes through the server, so the server is the
// only node; M5 lets devices issue their own. Values only need to order edits
// to the same field of the same row, so each module may keep its own Clock.
type Clock struct {
	mu      sync.Mutex
	clock   clock.Clock
	wall    int64
	counter int
}

const hlcNode = "server"

// New returns a Clock reading wall time from c.
func New(c clock.Clock) *Clock { return &Clock{clock: c} }

// Next returns a value greater than every value issued before.
func (h *Clock) Next() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.clock.Now().UnixMilli()
	if now > h.wall {
		h.wall, h.counter = now, 0
	} else {
		h.counter++
	}
	return fmt.Sprintf("%013d:%05d:%s", h.wall, h.counter, hlcNode)
}

// Stamp returns {"field": hlc, ...} for the changed fields, for field_hlc.
func (h *Clock) Stamp(fields ...string) []byte {
	v := h.Next()
	m := make(map[string]string, len(fields))
	for _, f := range fields {
		m[f] = v
	}
	b, _ := json.Marshal(m)
	return b
}
