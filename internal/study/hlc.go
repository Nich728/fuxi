package study

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Nich728/fuxi/internal/platform/clock"
)

// hlc issues hybrid logical clock values for edits made through the API
// (SYSTEM_DESIGN §7.3): {wall_ms:013d}:{counter:05d}:{node}. Until offline
// sync (M5) every edit comes through the server, so the server is the only
// node; M5 moves this to internal/sync and lets devices issue their own.
type hlc struct {
	mu      sync.Mutex
	clock   clock.Clock
	wall    int64
	counter int
}

const hlcNode = "server"

func newHLC(c clock.Clock) *hlc { return &hlc{clock: c} }

// next returns a value greater than every value issued before.
func (h *hlc) next() string {
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

// stamp returns {"field": hlc, ...} for the changed fields, for field_hlc.
func (h *hlc) stamp(fields ...string) []byte {
	v := h.next()
	m := make(map[string]string, len(fields))
	for _, f := range fields {
		m[f] = v
	}
	b, _ := json.Marshal(m)
	return b
}
