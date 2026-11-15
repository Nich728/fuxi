package study

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Nich728/fuxi/internal/scheduling"
	"github.com/Nich728/fuxi/internal/study/studydb"
)

// NewReview is a rating submitted from a device.
type NewReview struct {
	ID         uuid.UUID `json:"id"` // client-generated: the idempotency key (PRD FR-R-3)
	CardID     uuid.UUID `json:"card_id"`
	Rating     int       `json:"rating"` // 1 again, 2 hard, 3 good, 4 easy
	ReviewedAt time.Time `json:"reviewed_at"`
	DurationMs *int32    `json:"duration_ms"`
	DeviceID   uuid.UUID `json:"device_id"`
}

// maxClockAhead is how far in the future a device's review time may be.
// Phone clocks drift (SYSTEM_DESIGN §7.2); a review hours ahead is rejected
// rather than allowed to push the card's schedule into the future.
const maxClockAhead = 5 * time.Minute

// CardState is a card's schedule after a review.
type CardState struct {
	CardID       uuid.UUID  `json:"card_id"`
	State        string     `json:"state"`
	DueAt        time.Time  `json:"due_at"`
	Stability    float64    `json:"stability"`
	Difficulty   float64    `json:"difficulty"`
	Reps         int        `json:"reps"`
	Lapses       int        `json:"lapses"`
	LastReviewAt *time.Time `json:"last_review_at"`
}

// ReviewResult says what a submission did.
type ReviewResult struct {
	Card CardState `json:"card"`
	// Duplicate is true when this review id was already recorded: nothing changed.
	Duplicate bool `json:"duplicate"`
	// Replayed is true when the review was older than the card's last one,
	// so the whole log was replayed in order.
	Replayed bool `json:"replayed"`
}

// SubmitReview records a review and updates the card's schedule
// (SYSTEM_DESIGN §6.2), all in one transaction:
//  1. insert the review log row; if the id exists, it's a retry;
//  2. lock the card's state;
//  3. apply FSRS Next, or replay the whole log for a late (offline) review;
//  4. store the new state.
func (s *Service) SubmitReview(ctx context.Context, userID uuid.UUID, in NewReview) (ReviewResult, error) {
	if in.ID == uuid.Nil || in.CardID == uuid.Nil || in.DeviceID == uuid.Nil {
		return ReviewResult{}, invalid("id, card_id and device_id are required")
	}
	if in.Rating < scheduling.Again || in.Rating > scheduling.Easy {
		return ReviewResult{}, invalid("rating must be 1–4")
	}
	if in.DurationMs != nil && (*in.DurationMs < 0 || *in.DurationMs > 3_600_000) {
		return ReviewResult{}, invalid("duration_ms must be 0–3600000")
	}
	now := s.clock.Now()
	if in.ReviewedAt.IsZero() {
		return ReviewResult{}, invalid("reviewed_at is required")
	}
	if in.ReviewedAt.After(now.Add(maxClockAhead)) {
		return ReviewResult{}, invalid("reviewed_at is in the future; check the device clock")
	}
	in.ReviewedAt = in.ReviewedAt.UTC()

	card, err := s.q.GetCardForReview(ctx, studydb.GetCardForReviewParams{ID: in.CardID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewResult{}, ErrNotFound
	}
	if err != nil {
		return ReviewResult{}, fmt.Errorf("load card: %w", err)
	}
	ok, err := s.devices.OwnsDevice(ctx, userID, in.DeviceID)
	if err != nil {
		return ReviewResult{}, err
	}
	if !ok {
		return ReviewResult{}, invalid("unknown device_id; register it with PUT /v1/devices/{id}")
	}
	deck, err := parseDeckSettings(card.DeckSettings)
	if err != nil {
		return ReviewResult{}, err
	}
	params, err := s.params.UserParams(ctx, userID)
	if err != nil {
		return ReviewResult{}, err
	}
	params.DesiredRetention = deck.DesiredRetention

	var res ReviewResult
	err = s.inTx(ctx, func(q *studydb.Queries, _ pgx.Tx) error {
		_, err := q.InsertReviewLog(ctx, studydb.InsertReviewLogParams{
			ID: in.ID, UserID: userID, CardID: in.CardID, Rating: int16(in.Rating),
			ReviewedAt: in.ReviewedAt, DurationMs: in.DurationMs, DeviceID: in.DeviceID, ReceivedAt: now,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return s.duplicateReview(ctx, q, userID, in, &res)
		}
		if err != nil {
			return fmt.Errorf("insert review: %w", err)
		}

		row, err := q.LockCardState(ctx, studydb.LockCardStateParams{CardID: in.CardID, UserID: userID})
		if err != nil {
			return fmt.Errorf("lock card state: %w", err)
		}
		prev := fromRow(row)
		var next scheduling.State
		if prev.LastReview.IsZero() || !in.ReviewedAt.Before(prev.LastReview) {
			next, err = scheduling.Next(prev, in.Rating, in.ReviewedAt, params)
		} else {
			res.Replayed = true
			next, err = s.replay(ctx, q, userID, in.CardID, card.CreatedAt, params)
		}
		if err != nil {
			return fmt.Errorf("schedule: %w", err)
		}
		updated, err := q.UpdateCardState(ctx, toUpdate(userID, in.CardID, next, params.Version))
		if err != nil {
			return fmt.Errorf("update card state: %w", err)
		}
		res.Card = toCardState(updated)
		return nil
	})
	if err != nil {
		return ReviewResult{}, err
	}
	return res, nil
}

// duplicateReview handles a review id that is already in the log: a retry
// of the same review returns the current state; any other use is a clash.
func (s *Service) duplicateReview(ctx context.Context, q *studydb.Queries, userID uuid.UUID, in NewReview, res *ReviewResult) error {
	prior, err := q.GetReviewLogByID(ctx, in.ID)
	if err != nil {
		return fmt.Errorf("load prior review: %w", err)
	}
	if prior.UserID != userID || prior.CardID != in.CardID {
		return ErrConflict
	}
	row, err := q.GetCardState(ctx, studydb.GetCardStateParams{CardID: in.CardID, UserID: userID})
	if err != nil {
		return fmt.Errorf("load card state: %w", err)
	}
	res.Card, res.Duplicate = toCardState(row), true
	return nil
}

// replay rebuilds a card's state from its whole review log (ADR-0003).
func (s *Service) replay(ctx context.Context, q *studydb.Queries, userID, cardID uuid.UUID, created time.Time, p scheduling.Params) (scheduling.State, error) {
	rows, err := q.ListCardReviews(ctx, studydb.ListCardReviewsParams{CardID: cardID, UserID: userID})
	if err != nil {
		return scheduling.State{}, fmt.Errorf("load review log: %w", err)
	}
	reviews := make([]scheduling.Review, len(rows))
	for i, r := range rows {
		reviews[i] = scheduling.Review{ID: r.ID, Rating: int(r.Rating), ReviewedAt: r.ReviewedAt}
	}
	return scheduling.Replay(created, reviews, p)
}

// RebuildCard recomputes a card's state from its log and reports whether
// the stored state matched. It is the check behind "card state is derived"
// (SYSTEM_DESIGN §1.1) and the basis of the fsrs.rebuild job (M6).
func (s *Service) RebuildCard(ctx context.Context, userID, cardID uuid.UUID) (matched bool, err error) {
	card, err := s.q.GetCardForReview(ctx, studydb.GetCardForReviewParams{ID: cardID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("load card: %w", err)
	}
	deck, err := parseDeckSettings(card.DeckSettings)
	if err != nil {
		return false, err
	}
	params, err := s.params.UserParams(ctx, userID)
	if err != nil {
		return false, err
	}
	params.DesiredRetention = deck.DesiredRetention
	err = s.inTx(ctx, func(q *studydb.Queries, _ pgx.Tx) error {
		row, err := q.LockCardState(ctx, studydb.LockCardStateParams{CardID: cardID, UserID: userID})
		if err != nil {
			return fmt.Errorf("lock card state: %w", err)
		}
		next, err := s.replay(ctx, q, userID, cardID, card.CreatedAt, params)
		if err != nil {
			return err
		}
		matched = fromRow(row) == next
		if !matched {
			_, err = q.UpdateCardState(ctx, toUpdate(userID, cardID, next, params.Version))
		}
		return err
	})
	return matched, err
}

func fromRow(r studydb.CardState) scheduling.State {
	s := scheduling.State{
		State: r.State, Due: r.DueAt.UTC(), Stability: r.Stability, Difficulty: r.Difficulty,
		Reps: int(r.Reps), Lapses: int(r.Lapses), RemainingSteps: int(r.RemainingSteps),
	}
	if r.LastReviewAt != nil {
		s.LastReview = r.LastReviewAt.UTC()
	}
	if r.FirstReviewAt != nil {
		s.FirstReview = r.FirstReviewAt.UTC()
	}
	return s
}

func toUpdate(userID, cardID uuid.UUID, s scheduling.State, version int) studydb.UpdateCardStateParams {
	p := studydb.UpdateCardStateParams{
		CardID: cardID, UserID: userID, State: s.State, DueAt: s.Due,
		Stability: s.Stability, Difficulty: s.Difficulty, Reps: int32(s.Reps), Lapses: int32(s.Lapses),
		RemainingSteps: int32(s.RemainingSteps), ParamsVersion: int32(version),
	}
	if !s.LastReview.IsZero() {
		p.LastReviewAt = &s.LastReview
	}
	if !s.FirstReview.IsZero() {
		p.FirstReviewAt = &s.FirstReview
	}
	return p
}

func toCardState(r studydb.CardState) CardState {
	return CardState{
		CardID: r.CardID, State: stateName(r.State), DueAt: r.DueAt, Stability: r.Stability,
		Difficulty: r.Difficulty, Reps: int(r.Reps), Lapses: int(r.Lapses), LastReviewAt: r.LastReviewAt,
	}
}
