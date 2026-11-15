package study

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nich728/fuxi/internal/platform/clock"
	"github.com/Nich728/fuxi/internal/study/studydb"
)

// Service implements the study endpoints. Every method takes the user id
// and only touches that user's rows (SYSTEM_DESIGN §12.1).
type Service struct {
	pool    *pgxpool.Pool
	q       *studydb.Queries
	clock   clock.Clock
	hlc     *hlc
	dict    Dictionary
	devices Devices
	params  ParamsSource
}

// NewService returns a Service.
func NewService(pool *pgxpool.Pool, clk clock.Clock, dict Dictionary, devices Devices, params ParamsSource) *Service {
	return &Service{
		pool: pool, q: studydb.New(pool), clock: clk, hlc: newHLC(clk),
		dict: dict, devices: devices, params: params,
	}
}

// inTx runs fn in a transaction.
func (s *Service) inTx(ctx context.Context, fn func(q *studydb.Queries, tx pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(s.q.WithTx(tx), tx)
	})
}

// Decks ------------------------------------------------------------------

// CreateDeck creates a deck with a client-generated id. Repeating the call
// returns the existing deck (an idempotent retry).
func (s *Service) CreateDeck(ctx context.Context, userID, id uuid.UUID, name string, patch *DeckSettingsPatch) (Deck, error) {
	name, err := validDeckName(name)
	if err != nil {
		return Deck{}, err
	}
	settings, err := DefaultDeckSettings().apply(patch)
	if err != nil {
		return Deck{}, err
	}
	sb, err := json.Marshal(settings)
	if err != nil {
		return Deck{}, err
	}
	row, err := s.q.InsertDeck(ctx, studydb.InsertDeckParams{
		ID: id, UserID: userID, Name: name, Settings: sb,
		FieldHlc: s.hlc.stamp("name", "settings"), CreatedAt: s.clock.Now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return s.existingDeck(ctx, userID, id)
	}
	if err != nil {
		return Deck{}, fmt.Errorf("create deck: %w", err)
	}
	return toDeck(row.ID, row.Name, row.Settings, 0, row.CreatedAt)
}

// existingDeck resolves an insert conflict on a deck id.
func (s *Service) existingDeck(ctx context.Context, userID, id uuid.UUID) (Deck, error) {
	owner, err := s.q.DeckOwner(ctx, id)
	if err != nil {
		return Deck{}, fmt.Errorf("deck owner: %w", err)
	}
	if owner != userID {
		return Deck{}, ErrConflict
	}
	d, err := s.Deck(ctx, userID, id)
	if errors.Is(err, ErrNotFound) {
		return Deck{}, ErrConflict // the user deleted it; deletes are final (ADR-0006)
	}
	return d, err
}

// Deck returns one live deck.
func (s *Service) Deck(ctx context.Context, userID, id uuid.UUID) (Deck, error) {
	row, err := s.q.GetDeck(ctx, studydb.GetDeckParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Deck{}, ErrNotFound
	}
	if err != nil {
		return Deck{}, fmt.Errorf("load deck: %w", err)
	}
	return toDeck(row.ID, row.Name, row.Settings, 0, row.CreatedAt)
}

// Decks lists the user's live decks.
func (s *Service) Decks(ctx context.Context, userID uuid.UUID) ([]Deck, error) {
	rows, err := s.q.ListDecks(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list decks: %w", err)
	}
	out := make([]Deck, 0, len(rows))
	for _, r := range rows {
		d, err := toDeck(r.ID, r.Name, r.Settings, r.NoteCount, r.CreatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// UpdateDeck renames a deck or changes its settings. New settings apply to
// notes added later and to future reviews; existing cards are kept.
func (s *Service) UpdateDeck(ctx context.Context, userID, id uuid.UUID, name *string, patch *DeckSettingsPatch) (Deck, error) {
	cur, err := s.Deck(ctx, userID, id)
	if err != nil {
		return Deck{}, err
	}
	p := studydb.UpdateDeckParams{ID: id, UserID: userID}
	var changed []string
	if name != nil {
		n, err := validDeckName(*name)
		if err != nil {
			return Deck{}, err
		}
		p.Name = &n
		changed = append(changed, "name")
	}
	if patch != nil {
		settings, err := cur.Settings.apply(patch)
		if err != nil {
			return Deck{}, err
		}
		if p.Settings, err = json.Marshal(settings); err != nil {
			return Deck{}, err
		}
		changed = append(changed, "settings")
	}
	if len(changed) == 0 {
		return cur, nil
	}
	p.FieldHlc = s.hlc.stamp(changed...)
	row, err := s.q.UpdateDeck(ctx, p)
	if errors.Is(err, pgx.ErrNoRows) {
		return Deck{}, ErrNotFound
	}
	if err != nil {
		return Deck{}, fmt.Errorf("update deck: %w", err)
	}
	return toDeck(row.ID, row.Name, row.Settings, 0, row.CreatedAt)
}

// DeleteDeck deletes a deck with its notes and cards. The review log is
// kept (ADR-0003); deleted rows stay as tombstones for sync (ADR-0006).
func (s *Service) DeleteDeck(ctx context.Context, userID, id uuid.UUID) error {
	h := s.hlc.next()
	return s.inTx(ctx, func(q *studydb.Queries, _ pgx.Tx) error {
		n, err := q.DeleteDeck(ctx, studydb.DeleteDeckParams{ID: id, UserID: userID, Hlc: &h})
		if err != nil {
			return fmt.Errorf("delete deck: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		if err := q.DeleteDeckCards(ctx, studydb.DeleteDeckCardsParams{DeckID: id, UserID: userID, Hlc: &h}); err != nil {
			return fmt.Errorf("delete deck cards: %w", err)
		}
		if err := q.DeleteDeckNotes(ctx, studydb.DeleteDeckNotesParams{DeckID: id, UserID: userID, Hlc: &h}); err != nil {
			return fmt.Errorf("delete deck notes: %w", err)
		}
		if err := q.DeleteDeckCardStates(ctx, studydb.DeleteDeckCardStatesParams{DeckID: id, UserID: userID}); err != nil {
			return fmt.Errorf("delete deck card states: %w", err)
		}
		return nil
	})
}

func toDeck(id uuid.UUID, name string, settings []byte, notes int64, created time.Time) (Deck, error) {
	ds, err := parseDeckSettings(settings)
	if err != nil {
		return Deck{}, err
	}
	return Deck{ID: id, Name: name, Settings: ds, NoteCount: notes, CreatedAt: created}, nil
}

// Notes ------------------------------------------------------------------

// NewNote is a note to create.
type NewNote struct {
	ID      uuid.UUID `json:"id"`
	EntryID *int64    `json:"entry_id"`
	Kind    string    `json:"kind"` // word (default) | sentence
	Fields  Fields    `json:"fields"`
	Tags    []string  `json:"tags"`
	Origin  string    `json:"origin"` // default: search with an entry, manual without
}

// CreateNote adds a note and its cards to a deck. Repeating the call with
// the same id returns the existing note.
func (s *Service) CreateNote(ctx context.Context, userID, deckID uuid.UUID, n NewNote) (Note, error) {
	deck, err := s.Deck(ctx, userID, deckID)
	if err != nil {
		return Note{}, err
	}
	if err := s.prepareNote(ctx, &n); err != nil {
		return Note{}, err
	}
	params, err := s.params.UserParams(ctx, userID)
	if err != nil {
		return Note{}, err
	}
	var created *Note
	err = s.inTx(ctx, func(q *studydb.Queries, _ pgx.Tx) error {
		var err error
		created, err = s.insertNote(ctx, q, userID, deck, n, params.Version, s.clock.Now())
		return err
	})
	if err != nil {
		return Note{}, err
	}
	if created == nil {
		return s.existingNote(ctx, userID, n.ID)
	}
	return *created, nil
}

// prepareNote validates n and fills its facts from the dictionary.
func (s *Service) prepareNote(ctx context.Context, n *NewNote) error {
	if n.ID == uuid.Nil {
		return invalid("id is required")
	}
	if n.Kind == "" {
		n.Kind = "word"
	}
	if n.Kind != "word" && n.Kind != "sentence" {
		return invalid("kind must be word or sentence")
	}
	if n.Origin == "" {
		n.Origin = "manual"
		if n.EntryID != nil {
			n.Origin = "search"
		}
	}
	if !slices.Contains(origins, n.Origin) {
		return invalid("unknown origin %q", n.Origin)
	}
	if slices.Contains(dictionaryOrigins, n.Origin) && n.EntryID == nil {
		return invalid("origin %s needs an entry_id", n.Origin)
	}
	tags, err := validTags(n.Tags)
	if err != nil {
		return err
	}
	n.Tags = tags
	if n.EntryID != nil {
		e, err := s.dict.Entry(ctx, *n.EntryID)
		if errors.Is(err, ErrNoEntry) {
			return invalid("no dictionary entry %d", *n.EntryID)
		}
		if err != nil {
			return fmt.Errorf("look up entry: %w", err)
		}
		// The dictionary is the source of truth for facts (ADR-0009).
		n.Fields.Hanzi, n.Fields.Pinyin, n.Fields.Meaning = e.Hanzi, e.Pinyin, e.Meaning
	}
	n.Fields = trimFields(n.Fields)
	if n.Kind == "word" && (n.Fields.Hanzi == "" || n.Fields.Meaning == "") {
		return invalid("a word needs fields.hanzi and fields.meaning")
	}
	if n.Kind == "sentence" && n.Fields.Sentence == "" {
		return invalid("a sentence needs fields.sentence")
	}
	return n.Fields.validate()
}

func trimFields(f Fields) Fields {
	return Fields{
		Hanzi: strings.TrimSpace(f.Hanzi), Pinyin: strings.TrimSpace(f.Pinyin),
		Meaning: strings.TrimSpace(f.Meaning), Sentence: strings.TrimSpace(f.Sentence),
		Cloze: strings.TrimSpace(f.Cloze), Notes: strings.TrimSpace(f.Notes),
	}
}

// insertNote writes a prepared note with its cards and their new card
// states. It returns nil when the id already exists.
func (s *Service) insertNote(ctx context.Context, q *studydb.Queries, userID uuid.UUID, deck Deck, n NewNote, paramsVersion int, now time.Time) (*Note, error) {
	fb, err := json.Marshal(n.Fields)
	if err != nil {
		return nil, err
	}
	stamped := []string{"fields.hanzi", "fields.pinyin", "fields.meaning", "fields.sentence", "fields.cloze", "fields.notes", "tags"}
	row, err := q.InsertNote(ctx, studydb.InsertNoteParams{
		ID: n.ID, UserID: userID, DeckID: deck.ID, EntryID: n.EntryID, Kind: n.Kind,
		Fields: fb, Tags: n.Tags, Origin: n.Origin, FieldHlc: s.hlc.stamp(stamped...), CreatedAt: now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("insert note: %w", err)
	}
	note := toNote(row)
	for _, t := range cardTypesFor(n.Kind, n.Fields, deck.Settings.CardTypes) {
		id := CardID(n.ID, t)
		if err := q.InsertCard(ctx, studydb.InsertCardParams{
			ID: id, UserID: userID, NoteID: n.ID, CardType: t, FieldHlc: []byte("{}"), CreatedAt: now,
		}); err != nil {
			return nil, fmt.Errorf("insert card: %w", err)
		}
		if err := q.InsertCardState(ctx, studydb.InsertCardStateParams{
			CardID: id, UserID: userID, DeckID: deck.ID, DueAt: now, ParamsVersion: int32(paramsVersion),
		}); err != nil {
			return nil, fmt.Errorf("insert card state: %w", err)
		}
		note.Cards = append(note.Cards, Card{ID: id, Type: t, State: "new", DueAt: now.UTC()})
	}
	return &note, nil
}

func (s *Service) existingNote(ctx context.Context, userID, id uuid.UUID) (Note, error) {
	owner, err := s.q.NoteOwner(ctx, id)
	if err != nil {
		return Note{}, fmt.Errorf("note owner: %w", err)
	}
	if owner != userID {
		return Note{}, ErrConflict
	}
	n, err := s.Note(ctx, userID, id)
	if errors.Is(err, ErrNotFound) {
		return Note{}, ErrConflict
	}
	return n, err
}

// Note returns a live note with its cards.
func (s *Service) Note(ctx context.Context, userID, id uuid.UUID) (Note, error) {
	row, err := s.q.GetNote(ctx, studydb.GetNoteParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	if err != nil {
		return Note{}, fmt.Errorf("load note: %w", err)
	}
	n := toNote(row)
	cards, err := s.q.ListNoteCards(ctx, studydb.ListNoteCardsParams{NoteID: id, UserID: userID})
	if err != nil {
		return Note{}, fmt.Errorf("load cards: %w", err)
	}
	for _, c := range cards {
		n.Cards = append(n.Cards, Card{
			ID: c.ID, Type: c.CardType, Suspended: c.Suspended, State: stateName(c.State),
			DueAt: c.DueAt, Reps: int(c.Reps), Lapses: int(c.Lapses), LastReviewAt: c.LastReviewAt,
		})
	}
	return n, nil
}

// MaxPage bounds list pages.
const MaxPage = 200

// Notes lists a deck's live notes, oldest first, after the cursor (a note id).
func (s *Service) Notes(ctx context.Context, userID, deckID uuid.UUID, after uuid.UUID, limit int) ([]Note, error) {
	if _, err := s.Deck(ctx, userID, deckID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > MaxPage {
		limit = MaxPage
	}
	rows, err := s.q.ListNotes(ctx, studydb.ListNotesParams{DeckID: deckID, UserID: userID, After: after, Lim: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	out := make([]Note, len(rows))
	for i, r := range rows {
		out[i] = toNote(r)
	}
	return out, nil
}

// NotePatch edits a note; nil fields are kept. An empty string clears a field.
type NotePatch struct {
	Fields map[string]*string `json:"fields"`
	Tags   []string           `json:"tags"`
}

// UpdateNote edits a note's fields or tags. Each changed field gets a new
// HLC, so concurrent edits merge field by field in M5 (ADR-0005).
func (s *Service) UpdateNote(ctx context.Context, userID, id uuid.UUID, p NotePatch) (Note, error) {
	cur, err := s.Note(ctx, userID, id)
	if err != nil {
		return Note{}, err
	}
	merged := cur.Fields.asMap()
	set := map[string]string{}
	var changed []string
	for k, v := range p.Fields {
		if _, ok := merged[k]; !ok {
			return Note{}, invalid("unknown field %q", k)
		}
		if v == nil {
			continue
		}
		val := strings.TrimSpace(*v)
		merged[k], set[k] = val, val
		changed = append(changed, "fields."+k)
	}
	f := Fields{
		Hanzi: merged["hanzi"], Pinyin: merged["pinyin"], Meaning: merged["meaning"],
		Sentence: merged["sentence"], Cloze: merged["cloze"], Notes: merged["notes"],
	}
	if err := f.validate(); err != nil {
		return Note{}, err
	}
	if cur.Kind == "word" && (f.Hanzi == "" || f.Meaning == "") {
		return Note{}, invalid("a word needs hanzi and meaning")
	}
	if cur.Kind == "sentence" && f.Sentence == "" {
		return Note{}, invalid("a sentence needs a sentence")
	}
	var tags []string
	if p.Tags != nil {
		if tags, err = validTags(p.Tags); err != nil {
			return Note{}, err
		}
		changed = append(changed, "tags")
	}
	if len(changed) == 0 {
		return cur, nil
	}
	fb, err := json.Marshal(set)
	if err != nil {
		return Note{}, err
	}
	_, err = s.q.UpdateNote(ctx, studydb.UpdateNoteParams{
		ID: id, UserID: userID, Fields: fb, Tags: tags, FieldHlc: s.hlc.stamp(changed...),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	if err != nil {
		return Note{}, fmt.Errorf("update note: %w", err)
	}
	return s.Note(ctx, userID, id)
}

// DeleteNote deletes a note and its cards; their reviews are kept.
func (s *Service) DeleteNote(ctx context.Context, userID, id uuid.UUID) error {
	h := s.hlc.next()
	return s.inTx(ctx, func(q *studydb.Queries, _ pgx.Tx) error {
		n, err := q.DeleteNote(ctx, studydb.DeleteNoteParams{ID: id, UserID: userID, Hlc: &h})
		if err != nil {
			return fmt.Errorf("delete note: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		if err := q.DeleteNoteCardStates(ctx, studydb.DeleteNoteCardStatesParams{NoteID: id, UserID: userID}); err != nil {
			return fmt.Errorf("delete card states: %w", err)
		}
		if err := q.DeleteNoteCards(ctx, studydb.DeleteNoteCardsParams{NoteID: id, UserID: userID, Hlc: &h}); err != nil {
			return fmt.Errorf("delete cards: %w", err)
		}
		return nil
	})
}

func toNote(r studydb.Note) Note {
	var f Fields
	_ = json.Unmarshal(r.Fields, &f) // written by this package; always valid
	return Note{
		ID: r.ID, DeckID: r.DeckID, EntryID: r.EntryID, Kind: r.Kind, Fields: f,
		Tags: r.Tags, Origin: r.Origin, CreatedAt: r.CreatedAt,
	}
}

// Cards ------------------------------------------------------------------

// SetSuspended suspends a card or brings it back (PRD R7).
func (s *Service) SetSuspended(ctx context.Context, userID, cardID uuid.UUID, suspended bool) error {
	return s.inTx(ctx, func(q *studydb.Queries, _ pgx.Tx) error {
		n, err := q.SetCardSuspended(ctx, studydb.SetCardSuspendedParams{
			ID: cardID, UserID: userID, Suspended: suspended, FieldHlc: s.hlc.stamp("suspended"),
		})
		if err != nil {
			return fmt.Errorf("suspend card: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return q.SetCardStateSuspended(ctx, studydb.SetCardStateSuspendedParams{CardID: cardID, UserID: userID, Suspended: suspended})
	})
}
