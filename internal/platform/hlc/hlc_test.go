package hlc

import (
	"strings"
	"testing"
	"time"
)

type stepClock struct{ t time.Time }

func (c *stepClock) Now() time.Time { return c.t }

func TestNext(t *testing.T) {
	clk := &stepClock{t: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)}
	h := New(clk)
	a, b := h.Next(), h.Next()
	if a >= b {
		t.Errorf("%s not before %s", a, b)
	}
	clk.t = clk.t.Add(-time.Second) // the clock went backwards
	if c := h.Next(); c <= b {
		t.Errorf("%s not after %s after the clock went back", c, b)
	}
	if len(a) != len("0000000000000:00000:server") {
		t.Errorf("not fixed width: %q", a)
	}
}

func TestStamp(t *testing.T) {
	h := New(&stepClock{t: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)})
	got := string(h.Stamp("name", "settings"))
	if !strings.Contains(got, `"name":"1794614400000:00000:server"`) || !strings.Contains(got, `"settings":"1794614400000:00000:server"`) {
		t.Errorf("Stamp = %s", got)
	}
}
