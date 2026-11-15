package scheduling

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Nich728/fuxi/internal/scheduling/schedulingdb"
)

// Store reads a user's FSRS weights from fsrs_params.
type Store struct {
	q *schedulingdb.Queries
}

// NewStore returns a Store on db.
func NewStore(db schedulingdb.DBTX) *Store {
	return &Store{q: schedulingdb.New(db)}
}

// UserParams returns the user's newest weights, or the defaults (version 0)
// until the tuning job (M6) has stored some. DesiredRetention is per deck and
// left for the caller to set.
func (s *Store) UserParams(ctx context.Context, userID uuid.UUID) (Params, error) {
	row, err := s.q.LatestParams(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Params{}, nil
	}
	if err != nil {
		return Params{}, fmt.Errorf("load fsrs params: %w", err)
	}
	return Params{Version: int(row.Version), Weights: row.Weights}, nil
}
