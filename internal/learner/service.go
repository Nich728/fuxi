// Package learner owns what each user knows about individual words
// (SYSTEM_DESIGN §4.6, §9). In M3 that is "I already know this" (PRD V7);
// the vocabulary check, learner profile and Knowledge() arrive in M4.
package learner

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nich728/fuxi/internal/learner/learnerdb"
	"github.com/Nich728/fuxi/internal/platform/clock"
	"github.com/Nich728/fuxi/internal/platform/hlc"
)

// Errors returned by the Service.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid")
)

// Cards is what learner needs from internal/study: suspending or restoring
// a word's never-reviewed cards in the same transaction as the knowledge row.
type Cards interface {
	SetNewCardsSuspended(ctx context.Context, tx pgx.Tx, userID uuid.UUID, entryID int64, suspended bool) (int64, error)
}

// Knowledge is what a user knows about one dictionary entry.
type Knowledge struct {
	EntryID    int64     `json:"entry_id"`
	Status     string    `json:"status"` // known | learning | unknown
	Source     string    `json:"source"` // placement | manual | review
	Confidence float32   `json:"confidence"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MaxLookup bounds the entries asked about at once.
const MaxLookup = 200

// knowledgeNS is the namespace for word_knowledge ids: UUIDv5(user_id, entry_id)
// (SYSTEM_DESIGN §4.6), so every device derives the same id offline.
var knowledgeNS = uuid.MustParse("6f1c3e0a-5b7d-4c2e-9a41-2f8d0b6e7c53")

// KnowledgeID returns the word_knowledge id for a user and entry.
func KnowledgeID(userID uuid.UUID, entryID int64) uuid.UUID {
	return uuid.NewSHA1(knowledgeNS, append(userID[:], strconv.FormatInt(entryID, 10)...))
}

// Service implements the learner endpoints. Every method takes the user id
// and only touches that user's rows (SYSTEM_DESIGN §12.1).
type Service struct {
	pool  *pgxpool.Pool
	q     *learnerdb.Queries
	clock clock.Clock
	hlc   *hlc.Clock
	cards Cards
}

// NewService returns a Service.
func NewService(pool *pgxpool.Pool, clk clock.Clock, cards Cards) *Service {
	return &Service{pool: pool, q: learnerdb.New(pool), clock: clk, hlc: hlc.New(clk), cards: cards}
}

// Knowledge returns the user's rows for the given entries; entries with no
// row are left out (the client treats them as unknown until M4's Knowledge()).
func (s *Service) Knowledge(ctx context.Context, userID uuid.UUID, entryIDs []int64) ([]Knowledge, error) {
	if len(entryIDs) == 0 {
		return []Knowledge{}, nil
	}
	if len(entryIDs) > MaxLookup {
		return nil, fmt.Errorf("%w: at most %d entry ids", ErrInvalid, MaxLookup)
	}
	rows, err := s.q.ListKnowledge(ctx, learnerdb.ListKnowledgeParams{UserID: userID, EntryIds: entryIDs})
	if err != nil {
		return nil, fmt.Errorf("list knowledge: %w", err)
	}
	out := make([]Knowledge, len(rows))
	for i, r := range rows {
		out[i] = toKnowledge(r)
	}
	return out, nil
}

// MarkKnown is "I already know this" (SYSTEM_DESIGN §9.7): it records the word
// as known and suspends any of its cards that have never been reviewed, in one
// transaction. Repeating it changes nothing. It returns the cards suspended.
func (s *Service) MarkKnown(ctx context.Context, userID uuid.UUID, entryID int64) (Knowledge, int64, error) {
	var (
		k         Knowledge
		suspended int64
	)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpsertKnowledge(ctx, learnerdb.UpsertKnowledgeParams{
			ID: KnowledgeID(userID, entryID), UserID: userID, EntryID: entryID,
			Status: "known", Source: "manual", Confidence: 1,
			FieldHlc: s.hlc.Stamp("status"), UpdatedAt: s.clock.Now(),
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation: no such entry
				return ErrNotFound
			}
			return fmt.Errorf("upsert knowledge: %w", err)
		}
		k = toKnowledge(row)
		suspended, err = s.cards.SetNewCardsSuspended(ctx, tx, userID, entryID, true)
		return err
	})
	if err != nil {
		return Knowledge{}, 0, err
	}
	return k, suspended, nil
}

// Unmark undoes MarkKnown: it removes the user's row for the word and brings
// back the word's never-reviewed cards. A never-reviewed card the user had
// suspended by hand also comes back; M3 doesn't record why a card was
// suspended. It returns the cards restored.
func (s *Service) Unmark(ctx context.Context, userID uuid.UUID, entryID int64) (int64, error) {
	var restored int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).DeleteKnowledge(ctx, learnerdb.DeleteKnowledgeParams{UserID: userID, EntryID: entryID})
		if err != nil {
			return fmt.Errorf("delete knowledge: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		restored, err = s.cards.SetNewCardsSuspended(ctx, tx, userID, entryID, false)
		return err
	})
	return restored, err
}

func toKnowledge(r learnerdb.WordKnowledge) Knowledge {
	return Knowledge{EntryID: r.EntryID, Status: r.Status, Source: r.Source, Confidence: r.Confidence, UpdatedAt: r.UpdatedAt}
}
