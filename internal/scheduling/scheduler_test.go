package scheduling

import (
	"errors"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/google/uuid"
)

var t0 = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

func TestNextNewCard(t *testing.T) {
	tests := []struct {
		name      string
		rating    int
		wantState int16
		maxDue    time.Duration // due no later than this after the review
		minDue    time.Duration
	}{
		{"again stays in learning", Again, StateLearning, 2 * time.Minute, 0},
		{"good starts the next learning step", Good, StateLearning, 15 * time.Minute, time.Minute},
		{"easy graduates straight to review", Easy, StateReview, 30 * 24 * time.Hour, 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Next(NewState(t0), tt.rating, t0, Params{})
			if err != nil {
				t.Fatal(err)
			}
			if s.State != tt.wantState {
				t.Errorf("state = %d, want %d", s.State, tt.wantState)
			}
			if d := s.Due.Sub(t0); d < tt.minDue || d > tt.maxDue {
				t.Errorf("due in %v, want %v–%v", d, tt.minDue, tt.maxDue)
			}
			if s.Reps != 1 || !s.LastReview.Equal(t0) || !s.FirstReview.Equal(t0) {
				t.Errorf("reps=%d last=%v first=%v", s.Reps, s.LastReview, s.FirstReview)
			}
		})
	}
}

func TestNextRejects(t *testing.T) {
	if _, err := Next(NewState(t0), 0, t0, Params{}); !errors.Is(err, ErrInvalidRating) {
		t.Errorf("rating 0: err = %v", err)
	}
	if _, err := Next(NewState(t0), 5, t0, Params{}); !errors.Is(err, ErrInvalidRating) {
		t.Errorf("rating 5: err = %v", err)
	}
	s, err := Next(NewState(t0), Good, t0.Add(time.Hour), Params{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Next(s, Good, t0, Params{}); !errors.Is(err, ErrOutOfOrder) {
		t.Errorf("older review: err = %v", err)
	}
	if _, err := Next(NewState(t0), Good, t0, Params{Weights: []float64{1, 2}}); err == nil {
		t.Error("short weights accepted")
	}
}

func TestLapseCountsAndRetention(t *testing.T) {
	s := NewState(t0)
	at := t0
	for _, r := range []int{Good, Good, Good} {
		var err error
		if s, err = Next(s, r, at, Params{}); err != nil {
			t.Fatal(err)
		}
		at = s.Due
	}
	if s.State != StateReview {
		t.Fatalf("state after three goods = %d, want review", s.State)
	}
	lapsed, err := Next(s, Again, at, Params{})
	if err != nil {
		t.Fatal(err)
	}
	if lapsed.State != StateRelearning || lapsed.Lapses != 1 {
		t.Errorf("after again: state=%d lapses=%d", lapsed.State, lapsed.Lapses)
	}

	// A higher target retention means shorter intervals.
	lo, _ := Next(s, Good, at, Params{DesiredRetention: 0.8})
	hi, _ := Next(s, Good, at, Params{DesiredRetention: 0.97})
	if !hi.Due.Before(lo.Due) {
		t.Errorf("97%% due %v not before 80%% due %v", hi.Due, lo.Due)
	}
}

// Replay must equal applying Next one review at a time, whatever order the
// reviews arrive in (SYSTEM_DESIGN §6.1: replay is a fold over Next).
func TestReplayMatchesSequentialNext(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for run := range 200 {
		n := 1 + rng.IntN(12)
		reviews := make([]Review, n)
		at := t0.Add(time.Duration(rng.IntN(3600)) * time.Second)
		for i := range reviews {
			reviews[i] = Review{ID: uuid.New(), Rating: 1 + rng.IntN(4), ReviewedAt: at}
			// Gaps from seconds to weeks, sometimes zero (two reviews at once).
			at = at.Add(time.Duration(rng.IntN(4)) * time.Duration(rng.Int64N(int64(14*24*time.Hour))))
		}

		want := NewState(t0)
		inOrder := make([]Review, n)
		copy(inOrder, reviews)
		sortReviews(inOrder)
		for _, r := range inOrder {
			var err error
			if want, err = Next(want, r.Rating, r.ReviewedAt, Params{}); err != nil {
				t.Fatalf("run %d: %v", run, err)
			}
		}

		rng.Shuffle(n, func(i, j int) { reviews[i], reviews[j] = reviews[j], reviews[i] })
		got, err := Replay(t0, reviews, Params{})
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if got != want {
			t.Fatalf("run %d: replay = %+v\nwant %+v", run, got, want)
		}
		again, _ := Replay(t0, reviews, Params{})
		if again != got {
			t.Fatalf("run %d: replay is not deterministic", run)
		}
	}
}

func TestReplayEmpty(t *testing.T) {
	s, err := Replay(t0, nil, Params{})
	if err != nil {
		t.Fatal(err)
	}
	if s != NewState(t0) {
		t.Errorf("empty replay = %+v", s)
	}
}

func sortReviews(rs []Review) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0; j-- {
			a, b := rs[j-1], rs[j]
			if a.ReviewedAt.After(b.ReviewedAt) ||
				(a.ReviewedAt.Equal(b.ReviewedAt) && a.ID.String() > b.ID.String()) {
				rs[j-1], rs[j] = b, a
			}
		}
	}
}
