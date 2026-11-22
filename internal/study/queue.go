package study

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Nich728/fuxi/internal/platform/principal"
	"github.com/Nich728/fuxi/internal/scheduling"
	"github.com/Nich728/fuxi/internal/study/studydb"
)

// QueueCard is a card to review, with what the client needs to show it.
type QueueCard struct {
	CardID  uuid.UUID `json:"card_id"`
	Type    string    `json:"type"`
	State   string    `json:"state"`
	DueAt   time.Time `json:"due_at"`
	Reps    int       `json:"reps"`
	Lapses  int       `json:"lapses"`
	DeckID  uuid.UUID `json:"deck_id"`
	NoteID  uuid.UUID `json:"note_id"`
	Kind    string    `json:"kind"`
	EntryID *int64    `json:"entry_id"`
	Fields  Fields    `json:"fields"`
	// Preview is when the card would next be due for each rating, if rated
	// now, so the rating buttons can show their intervals (design system:
	// review screen). The client formats them relative to the time it shows.
	Preview RatingPreview `json:"preview"`

	state scheduling.State // for Preview
}

// RatingPreview holds the next due time for each rating.
type RatingPreview struct {
	Again time.Time `json:"again"`
	Hard  time.Time `json:"hard"`
	Good  time.Time `json:"good"`
	Easy  time.Time `json:"easy"`
}

// Queue is what to study now.
type Queue struct {
	DayStart time.Time   `json:"day_start"`
	DayEnd   time.Time   `json:"day_end"`
	Due      []QueueCard `json:"due"`       // oldest due first
	New      []QueueCard `json:"new"`       // today's allowance of new cards
	DueCount int64       `json:"due_count"` // all due today, beyond the page (PRD R1)
	NewCount int64       `json:"new_count"` // new cards still allowed today
}

// MaxQueue bounds the due cards returned at once.
const MaxQueue = 500

// Queue returns the due cards and today's new cards (SYSTEM_DESIGN §6.3),
// optionally for one deck. Due means due before the end of the user's study
// day, so a card due at 3 pm shows from the morning.
func (s *Service) Queue(ctx context.Context, p principal.Principal, deckID *uuid.UUID, limit int) (Queue, error) {
	loc, err := location(p.Timezone)
	if err != nil {
		return Queue{}, err
	}
	start, end := DayBounds(s.clock.Now(), loc, p.DayStartsAt)
	if limit <= 0 || limit > MaxQueue {
		limit = MaxQueue
	}
	deck := uuid.NullUUID{}
	if deckID != nil {
		if _, err := s.Deck(ctx, p.UserID, *deckID); err != nil {
			return Queue{}, err
		}
		deck = uuid.NullUUID{UUID: *deckID, Valid: true}
	}

	q := Queue{DayStart: start, DayEnd: end, Due: []QueueCard{}, New: []QueueCard{}}
	if q.DueCount, err = s.q.CountDue(ctx, studydb.CountDueParams{UserID: p.UserID, DayEnd: end, DeckID: deck}); err != nil {
		return Queue{}, fmt.Errorf("count due: %w", err)
	}
	due, err := s.q.ListDue(ctx, studydb.ListDueParams{UserID: p.UserID, DayEnd: end, DeckID: deck, Lim: int32(limit)})
	if err != nil {
		return Queue{}, fmt.Errorf("list due: %w", err)
	}
	for _, r := range due {
		q.Due = append(q.Due, queueCard(r))
	}

	// New cards: each deck's new_per_day minus those started today.
	allowances, err := s.newAllowances(ctx, p.UserID, deck, start)
	if err != nil {
		return Queue{}, err
	}
	for _, d := range allowances {
		q.NewCount += d.allowed
		rows, err := s.q.ListNew(ctx, studydb.ListNewParams{UserID: p.UserID, DeckID: d.id, Lim: int32(min(d.allowed, MaxQueue))})
		if err != nil {
			return Queue{}, fmt.Errorf("list new: %w", err)
		}
		for _, r := range rows {
			q.New = append(q.New, queueCard(studydb.ListDueRow(r)))
		}
	}

	if err := s.preview(ctx, p.UserID, q.Due, q.New); err != nil {
		return Queue{}, err
	}
	return q, nil
}

type deckAllowance struct {
	id        uuid.UUID
	settings  DeckSettings
	allowed   int64 // new cards still allowed today; may be 0
	available int64 // new cards in the deck
}

// newAllowances returns each live deck (or just deck) with how many new
// cards it may still introduce today: new_per_day minus the cards whose first
// review fell in this study day, capped by the new cards it has (§6.3).
func (s *Service) newAllowances(ctx context.Context, userID uuid.UUID, deck uuid.NullUUID, dayStart time.Time) ([]deckAllowance, error) {
	decks, err := s.q.ListQueueDecks(ctx, studydb.ListQueueDecksParams{UserID: userID, DeckID: deck})
	if err != nil {
		return nil, fmt.Errorf("list decks: %w", err)
	}
	introducedRows, err := s.q.CountIntroduced(ctx, studydb.CountIntroducedParams{UserID: userID, DayStart: dayStart})
	if err != nil {
		return nil, fmt.Errorf("count introduced: %w", err)
	}
	introduced := map[uuid.UUID]int64{}
	for _, r := range introducedRows {
		introduced[r.DeckID] = r.N
	}
	newRows, err := s.q.CountNew(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count new: %w", err)
	}
	available := map[uuid.UUID]int64{}
	for _, r := range newRows {
		available[r.DeckID] = r.N
	}
	out := make([]deckAllowance, 0, len(decks))
	for _, d := range decks {
		settings, err := parseDeckSettings(d.Settings)
		if err != nil {
			return nil, err
		}
		allowed := max(0, min(int64(settings.NewPerDay)-introduced[d.ID], available[d.ID]))
		out = append(out, deckAllowance{id: d.ID, settings: settings, allowed: allowed, available: available[d.ID]})
	}
	return out, nil
}

// preview fills in each card's RatingPreview, using the user's FSRS weights
// and each deck's target retention, as if rated now.
func (s *Service) preview(ctx context.Context, userID uuid.UUID, lists ...[]QueueCard) error {
	n := 0
	for _, l := range lists {
		n += len(l)
	}
	if n == 0 {
		return nil
	}
	params, err := s.params.UserParams(ctx, userID)
	if err != nil {
		return err
	}
	decks, err := s.q.ListQueueDecks(ctx, studydb.ListQueueDecksParams{UserID: userID})
	if err != nil {
		return fmt.Errorf("list decks: %w", err)
	}
	retention := make(map[uuid.UUID]float64, len(decks))
	for _, d := range decks {
		settings, err := parseDeckSettings(d.Settings)
		if err != nil {
			return err
		}
		retention[d.ID] = settings.DesiredRetention
	}
	now := s.clock.Now()
	for _, l := range lists {
		for i := range l {
			c := &l[i]
			p := params
			p.DesiredRetention = retention[c.DeckID]
			var due [4]time.Time
			for r := scheduling.Again; r <= scheduling.Easy; r++ {
				at := now
				if at.Before(c.state.LastReview) {
					at = c.state.LastReview // a review stored with a slightly fast device clock
				}
				next, err := scheduling.Next(c.state, r, at, p)
				if err != nil {
					return fmt.Errorf("preview card %s: %w", c.CardID, err)
				}
				due[r-1] = next.Due
			}
			c.Preview = RatingPreview{Again: due[0], Hard: due[1], Good: due[2], Easy: due[3]}
		}
	}
	return nil
}

func queueCard(r studydb.ListDueRow) QueueCard {
	var f Fields
	_ = json.Unmarshal(r.Fields, &f)
	st := scheduling.State{
		State: r.State, Due: r.DueAt.UTC(), Stability: r.Stability, Difficulty: r.Difficulty,
		Reps: int(r.Reps), Lapses: int(r.Lapses), RemainingSteps: int(r.RemainingSteps),
	}
	if r.LastReviewAt != nil {
		st.LastReview = r.LastReviewAt.UTC()
	}
	if r.FirstReviewAt != nil {
		st.FirstReview = r.FirstReviewAt.UTC()
	}
	return QueueCard{
		CardID: r.CardID, Type: r.CardType, State: stateName(r.State), DueAt: r.DueAt, Reps: int(r.Reps), Lapses: int(r.Lapses),
		DeckID: r.DeckID, NoteID: r.NoteID, Kind: r.Kind, EntryID: r.EntryID, Fields: f, state: st,
	}
}
