package learner

import (
	"testing"

	"github.com/google/uuid"
)

// Every device must derive the same word_knowledge id offline (SYSTEM_DESIGN §4.6).
func TestKnowledgeID(t *testing.T) {
	u1 := uuid.MustParse("0193a000-0000-7000-8000-000000000001")
	u2 := uuid.MustParse("0193a000-0000-7000-8000-000000000002")
	a := KnowledgeID(u1, 42)
	if a != KnowledgeID(u1, 42) {
		t.Fatal("not deterministic")
	}
	if a.Version() != 5 {
		t.Errorf("version %d, want 5", a.Version())
	}
	for _, other := range []uuid.UUID{KnowledgeID(u2, 42), KnowledgeID(u1, 43), KnowledgeID(u1, 420)} {
		if other == a {
			t.Errorf("collision: %s", other)
		}
	}
}
