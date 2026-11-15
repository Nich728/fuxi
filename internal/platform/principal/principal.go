// Package principal carries the signed-in user through a request's context.
//
// internal/identity sets it after verifying the request; other modules read
// it to scope their queries (SYSTEM_DESIGN §12.1) without depending on
// identity's tables.
package principal

import (
	"context"

	"github.com/google/uuid"
)

// Principal is the user a request acts for.
type Principal struct {
	UserID      uuid.UUID
	Email       string
	Role        string // member | owner
	Timezone    string // IANA name, e.g. Asia/Jakarta
	DayStartsAt int    // hour the study day starts (SYSTEM_DESIGN §6.4)
}

type key struct{}

// With returns a context carrying p.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, key{}, p)
}

// From returns the principal in ctx, if any.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(key{}).(Principal)
	return p, ok
}
