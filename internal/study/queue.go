package study

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Nich728/fuxi/internal/platform/principal"
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
		q.Due = append(q.Due, queueCard(r.CardID, r.CardType, r.State, r.DueAt, r.Reps, r.Lapses, r.DeckID, r.NoteID, r.Kind, r.EntryID, r.Fields))
	}

	// New cards: each deck's new_per_day minus those started today.
	decks, err := s.q.ListQueueDecks(ctx, studydb.ListQueueDecksParams{UserID: p.UserID, DeckID: deck})
	if err != nil {
		return Queue{}, fmt.Errorf("list decks: %w", err)
	}
	introducedRows, err := s.q.CountIntroduced(ctx, studydb.CountIntroducedParams{UserID: p.UserID, DayStart: start})
	if err != nil {
		return Queue{}, fmt.Errorf("count introduced: %w", err)
	}
	introduced := map[uuid.UUID]int64{}
	for _, r := range introducedRows {
		introduced[r.DeckID] = r.N
	}
	newRows, err := s.q.CountNew(ctx, p.UserID)
	if err != nil {
		return Queue{}, fmt.Errorf("count new: %w", err)
	}
	available := map[uuid.UUID]int64{}
	for _, r := range newRows {
		available[r.DeckID] = r.N
	}
	for _, d := range decks {
		var settings DeckSettings
		if settings, err = parseDeckSettings(d.Settings); err != nil {
			return Queue{}, err
		}
		allowed := min(int64(settings.NewPerDay)-introduced[d.ID], available[d.ID])
		if allowed <= 0 {
			continue
		}
		q.NewCount += allowed
		rows, err := s.q.ListNew(ctx, studydb.ListNewParams{UserID: p.UserID, DeckID: d.ID, Lim: int32(min(allowed, MaxQueue))})
		if err != nil {
			return Queue{}, fmt.Errorf("list new: %w", err)
		}
		for _, r := range rows {
			q.New = append(q.New, queueCard(r.CardID, r.CardType, r.State, r.DueAt, r.Reps, r.Lapses, r.DeckID, r.NoteID, r.Kind, r.EntryID, r.Fields))
		}
	}
	return q, nil
}

func queueCard(cardID uuid.UUID, typ string, state int16, due time.Time, reps, lapses int32,
	deckID, noteID uuid.UUID, kind string, entryID *int64, fields []byte) QueueCard {
	var f Fields
	_ = json.Unmarshal(fields, &f)
	return QueueCard{
		CardID: cardID, Type: typ, State: stateName(state), DueAt: due, Reps: int(reps), Lapses: int(lapses),
		DeckID: deckID, NoteID: noteID, Kind: kind, EntryID: entryID, Fields: f,
	}
}
