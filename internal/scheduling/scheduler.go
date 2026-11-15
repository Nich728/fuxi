// Package scheduling wraps go-fsrs (SYSTEM_DESIGN §6.1).
//
// Next and Replay are pure: no clock, no I/O. go-fsrs v4 leaves fuzz off by
// default and the wrapper never turns it on, so the same reviews always give
// the same state (this answers SYSTEM_DESIGN §16 question 1).
package scheduling

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

// Card states, as stored in card_states.state.
const (
	StateNew        = int16(fsrs.New)
	StateLearning   = int16(fsrs.Learning)
	StateReview     = int16(fsrs.Review)
	StateRelearning = int16(fsrs.Relearning)
)

// Ratings, as stored in review_logs.rating.
const (
	Again = 1
	Hard  = 2
	Good  = 3
	Easy  = 4
)

// DefaultRetention is the target retention when a deck doesn't set one (PRD FR-R-1).
const DefaultRetention = 0.9

// ErrInvalidRating is returned for a rating outside 1–4.
var ErrInvalidRating = errors.New("rating must be 1 (again) to 4 (easy)")

// ErrOutOfOrder is returned by Next when the review is older than the
// card's last review; the caller should Replay the whole log instead.
var ErrOutOfOrder = errors.New("review is older than the card's last review")

// State is a card's scheduling state: the cached row in card_states.
type State struct {
	State          int16
	Due            time.Time
	Stability      float64
	Difficulty     float64
	Reps           int
	Lapses         int
	RemainingSteps int
	LastReview     time.Time // zero until the first review
	FirstReview    time.Time // zero until the first review
}

// Review is one entry of the review log.
type Review struct {
	ID         uuid.UUID
	Rating     int
	ReviewedAt time.Time
}

// Params selects the FSRS model for a user and deck.
type Params struct {
	// Version is fsrs_params.version; 0 means the FSRS default weights.
	Version int
	// Weights are tuned weights (SYSTEM_DESIGN §6.5); nil means the defaults.
	Weights []float64
	// DesiredRetention is the deck's target, 0.80–0.97; 0 means DefaultRetention.
	DesiredRetention float64
}

// NewState is the state of a card that has never been reviewed. It is due
// from the moment it was created.
func NewState(created time.Time) State {
	return State{State: StateNew, Due: dbTime(created)}
}

// Next returns the state after one review.
func Next(prev State, rating int, at time.Time, p Params) (State, error) {
	if rating < Again || rating > Easy {
		return State{}, ErrInvalidRating
	}
	if !prev.LastReview.IsZero() && at.Before(prev.LastReview) {
		return State{}, ErrOutOfOrder
	}
	f, err := model(p)
	if err != nil {
		return State{}, err
	}
	at = dbTime(at)
	info, err := f.Next(toCard(prev), at, fsrs.Rating(rating))
	if err != nil {
		return State{}, fmt.Errorf("fsrs next: %w", err)
	}
	next := fromCard(info.Card)
	next.FirstReview = prev.FirstReview
	if next.FirstReview.IsZero() {
		next.FirstReview = at
	}
	return next, nil
}

// Replay folds a card's reviews into a state, oldest first by
// (reviewed_at, id), so the result doesn't depend on arrival order. It is
// how late offline reviews and new parameters are applied.
func Replay(created time.Time, reviews []Review, p Params) (State, error) {
	sorted := make([]Review, len(reviews))
	copy(sorted, reviews)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if !a.ReviewedAt.Equal(b.ReviewedAt) {
			return a.ReviewedAt.Before(b.ReviewedAt)
		}
		return a.ID.String() < b.ID.String()
	})
	s := NewState(created)
	for _, r := range sorted {
		var err error
		if s, err = Next(s, r.Rating, r.ReviewedAt, p); err != nil {
			return State{}, fmt.Errorf("replay review %s: %w", r.ID, err)
		}
	}
	return s, nil
}

func model(p Params) (*fsrs.FSRS, error) {
	fp := fsrs.DefaultParam()
	if p.DesiredRetention != 0 {
		if p.DesiredRetention < 0.7 || p.DesiredRetention > 0.99 {
			return nil, fmt.Errorf("desired retention %v out of range", p.DesiredRetention)
		}
		fp.RequestRetention = p.DesiredRetention
	}
	if p.Weights != nil {
		if len(p.Weights) != len(fp.W) {
			return nil, fmt.Errorf("fsrs weights: got %d, want %d", len(p.Weights), len(fp.W))
		}
		copy(fp.W[:], p.Weights)
	}
	fp.EnableFuzz = false // deterministic replay (SYSTEM_DESIGN §6.1)
	return fsrs.NewFSRS(fp), nil
}

func toCard(s State) fsrs.Card {
	return fsrs.Card{
		Due:            s.Due,
		Stability:      s.Stability,
		Difficulty:     s.Difficulty,
		Reps:           uint64(s.Reps),
		Lapses:         uint64(s.Lapses),
		State:          fsrs.State(s.State),
		LastReview:     s.LastReview,
		RemainingSteps: s.RemainingSteps,
	}
}

func fromCard(c fsrs.Card) State {
	return State{
		State:          int16(c.State),
		Due:            dbTime(c.Due),
		Stability:      c.Stability,
		Difficulty:     c.Difficulty,
		Reps:           int(c.Reps),
		Lapses:         int(c.Lapses),
		RemainingSteps: c.RemainingSteps,
		LastReview:     dbTime(c.LastReview),
	}
}

// dbTime rounds to Postgres's microsecond precision, so a state read back
// from card_states equals the one computed in memory and Next on a stored
// state matches Replay exactly.
func dbTime(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC().Truncate(time.Microsecond)
}
