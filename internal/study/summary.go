package study

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Nich728/fuxi/internal/platform/principal"
	"github.com/Nich728/fuxi/internal/study/studydb"
)

// Summary is what the home screen shows (SYSTEM_DESIGN §6.6, PRD R1).
type Summary struct {
	DayStart time.Time `json:"day_start"`
	DayEnd   time.Time `json:"day_end"`
	Today    string    `json:"today"` // the study day's date in the user's timezone, YYYY-MM-DD
	DueCount int64     `json:"due_count"`
	NewCount int64     `json:"new_count"`
	// Decks has one row per live deck, oldest first.
	Decks []DeckSummary `json:"decks"`
	// Days is the last SummaryDays study days, oldest first, ending today:
	// the seal week on the home screen.
	Days []DaySummary `json:"days"`
}

// DeckSummary is one deck's counts for today.
type DeckSummary struct {
	DeckID uuid.UUID `json:"deck_id"`
	Due    int64     `json:"due"`
	New    int64     `json:"new"` // new cards still allowed today
}

// DaySummary is one study day. A day with at least one review is sealed.
type DaySummary struct {
	Date    string `json:"date"`
	Reviews int64  `json:"reviews"`
}

// SummaryDays is the length of the seal week.
const SummaryDays = 7

// Summary returns today's counts per deck and reviews per day for the last week.
func (s *Service) Summary(ctx context.Context, p principal.Principal) (Summary, error) {
	loc, err := location(p.Timezone)
	if err != nil {
		return Summary{}, err
	}
	start, end := DayBounds(s.clock.Now(), loc, p.DayStartsAt)
	local := start.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	sum := Summary{DayStart: start, DayEnd: end, Today: today.Format(time.DateOnly), Decks: []DeckSummary{}}

	allowances, err := s.newAllowances(ctx, p.UserID, uuid.NullUUID{}, start)
	if err != nil {
		return Summary{}, err
	}
	dueRows, err := s.q.CountDueByDeck(ctx, studydb.CountDueByDeckParams{UserID: p.UserID, DayEnd: end})
	if err != nil {
		return Summary{}, fmt.Errorf("count due: %w", err)
	}
	due := make(map[uuid.UUID]int64, len(dueRows))
	for _, r := range dueRows {
		due[r.DeckID] = r.N
	}
	for _, d := range allowances {
		sum.Decks = append(sum.Decks, DeckSummary{DeckID: d.id, Due: due[d.id], New: d.allowed})
		sum.DueCount += due[d.id]
		sum.NewCount += d.allowed
	}

	// The week starts SummaryDays-1 study days before today. since only
	// bounds the scan: SQL assigns each review its study date, and dates
	// outside the week are ignored, so a DST hour either way doesn't matter.
	first := today.AddDate(0, 0, -(SummaryDays - 1))
	since := local.AddDate(0, 0, -(SummaryDays - 1))
	rows, err := s.q.ReviewsPerDay(ctx, studydb.ReviewsPerDayParams{
		Tz: loc.String(), DayStartsAt: int32(p.DayStartsAt), UserID: p.UserID, Since: since,
	})
	if err != nil {
		return Summary{}, fmt.Errorf("reviews per day: %w", err)
	}
	perDay := make(map[string]int64, len(rows))
	for _, r := range rows {
		perDay[r.Day.Time.Format(time.DateOnly)] = r.N
	}
	for i := range SummaryDays {
		d := first.AddDate(0, 0, i).Format(time.DateOnly)
		sum.Days = append(sum.Days, DaySummary{Date: d, Reviews: perDay[d]})
	}
	return sum, nil
}

// NotesForEntry lists the user's live notes for a dictionary entry, so the
// word page can say which decks already have it.
func (s *Service) NotesForEntry(ctx context.Context, userID uuid.UUID, entryID int64) ([]Note, error) {
	rows, err := s.q.ListNotesByEntry(ctx, studydb.ListNotesByEntryParams{UserID: userID, EntryID: &entryID})
	if err != nil {
		return nil, fmt.Errorf("list notes for entry: %w", err)
	}
	out := make([]Note, len(rows))
	for i, r := range rows {
		out[i] = toNote(r)
	}
	return out, nil
}

// SetNewCardsSuspended suspends, or brings back, the never-reviewed cards of
// every note for a dictionary entry, inside the caller's transaction. It is
// how internal/learner applies "I already know this" (SYSTEM_DESIGN §9.7)
// without touching study's tables itself. It returns the cards changed.
func (s *Service) SetNewCardsSuspended(ctx context.Context, tx pgx.Tx, userID uuid.UUID, entryID int64, suspended bool) (int64, error) {
	n, err := s.q.WithTx(tx).SetNewCardsSuspendedForEntry(ctx, studydb.SetNewCardsSuspendedForEntryParams{
		Suspended: suspended, UserID: userID, FieldHlc: s.hlc.Stamp("suspended"), EntryID: &entryID,
	})
	if err != nil {
		return 0, fmt.Errorf("suspend new cards: %w", err)
	}
	return n, nil
}
