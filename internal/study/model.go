// Package study owns decks, notes, cards, the review log and card state
// (SYSTEM_DESIGN §4.3, §6.2–§6.4).
package study

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Nich728/fuxi/internal/scheduling"
)

// Errors returned by Service. The HTTP layer maps them to status codes.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid input")
	// ErrConflict means a client-generated id is already used for something else.
	ErrConflict = errors.New("id already used")
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// Card types (PRD R6).
const (
	Recognition = "recognition" // hanzi → meaning
	Recall      = "recall"      // meaning → hanzi
	Tones       = "tones"       // hanzi → pinyin with tones
	Cloze       = "cloze"       // sentence with a gap
)

var cardTypes = []string{Recognition, Recall, Tones, Cloze}

// Origins say how a note was created (SYSTEM_DESIGN §4.3).
var (
	origins = []string{"search", "csv", "hsk", "tutor", "extract", "placement", "manual"}
	// These come from the dictionary or the AI, so the note must name a
	// dictionary entry and its facts are copied from there (ADR-0009).
	dictionaryOrigins = []string{"search", "hsk", "tutor", "extract", "placement"}
)

// DeckSettings are per-deck study options.
type DeckSettings struct {
	NewPerDay        int      `json:"new_per_day"`       // PRD R4
	CardTypes        []string `json:"card_types"`        // PRD R6
	DesiredRetention float64  `json:"desired_retention"` // PRD FR-R-1: 0.80–0.97
}

// DefaultDeckSettings are used for fields a new deck doesn't set.
func DefaultDeckSettings() DeckSettings {
	return DeckSettings{NewPerDay: 20, CardTypes: []string{Recognition, Recall}, DesiredRetention: scheduling.DefaultRetention}
}

// DeckSettingsPatch overrides some settings; nil fields are kept.
type DeckSettingsPatch struct {
	NewPerDay        *int     `json:"new_per_day"`
	CardTypes        []string `json:"card_types"`
	DesiredRetention *float64 `json:"desired_retention"`
}

func (s DeckSettings) apply(p *DeckSettingsPatch) (DeckSettings, error) {
	if p == nil {
		return s, nil
	}
	if p.NewPerDay != nil {
		s.NewPerDay = *p.NewPerDay
	}
	if p.CardTypes != nil {
		s.CardTypes = p.CardTypes
	}
	if p.DesiredRetention != nil {
		s.DesiredRetention = *p.DesiredRetention
	}
	return s, s.validate()
}

func (s DeckSettings) validate() error {
	if s.NewPerDay < 0 || s.NewPerDay > 1000 {
		return invalid("new_per_day must be 0–1000")
	}
	if len(s.CardTypes) == 0 {
		return invalid("card_types must not be empty")
	}
	for i, t := range s.CardTypes {
		if !slices.Contains(cardTypes, t) {
			return invalid("unknown card type %q", t)
		}
		if slices.Contains(s.CardTypes[:i], t) {
			return invalid("card type %q listed twice", t)
		}
	}
	if s.DesiredRetention < 0.80 || s.DesiredRetention > 0.97 {
		return invalid("desired_retention must be 0.80–0.97")
	}
	return nil
}

func parseDeckSettings(b []byte) (DeckSettings, error) {
	s := DefaultDeckSettings()
	if err := json.Unmarshal(b, &s); err != nil {
		return DeckSettings{}, fmt.Errorf("decode deck settings: %w", err)
	}
	return s, nil
}

// Deck is a named collection of notes.
type Deck struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Settings  DeckSettings `json:"settings"`
	NoteCount int64        `json:"note_count"`
	CreatedAt time.Time    `json:"created_at"`
}

func validDeckName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return "", invalid("name must be 1–100 characters")
	}
	return name, nil
}

// Fields are a note's content. For a note linked to the dictionary, hanzi,
// pinyin and meaning are copied from the entry when the note is created.
type Fields struct {
	Hanzi    string `json:"hanzi,omitempty"`
	Pinyin   string `json:"pinyin,omitempty"`
	Meaning  string `json:"meaning,omitempty"`
	Sentence string `json:"sentence,omitempty"`
	Cloze    string `json:"cloze,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// maxField bounds each field; a note is a flashcard, not a document.
const maxField = 2000

func (f Fields) validate() error {
	for name, v := range f.asMap() {
		if utf8.RuneCountInString(v) > maxField {
			return invalid("%s longer than %d characters", name, maxField)
		}
	}
	return nil
}

func (f Fields) asMap() map[string]string {
	return map[string]string{
		"hanzi": f.Hanzi, "pinyin": f.Pinyin, "meaning": f.Meaning,
		"sentence": f.Sentence, "cloze": f.Cloze, "notes": f.Notes,
	}
}

func validTags(tags []string) ([]string, error) {
	if len(tags) > 20 {
		return nil, invalid("at most 20 tags")
	}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || utf8.RuneCountInString(t) > 50 || strings.ContainsAny(t, " \t\n") {
			return nil, invalid("tags must be 1–50 characters without spaces")
		}
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out, nil
}

// Note is one thing to learn: a word or a sentence.
type Note struct {
	ID        uuid.UUID `json:"id"`
	DeckID    uuid.UUID `json:"deck_id"`
	EntryID   *int64    `json:"entry_id"`
	Kind      string    `json:"kind"` // word | sentence
	Fields    Fields    `json:"fields"`
	Tags      []string  `json:"tags"`
	Origin    string    `json:"origin"`
	CreatedAt time.Time `json:"created_at"`
	Cards     []Card    `json:"cards,omitempty"`
}

// Card is one way of reviewing a note.
type Card struct {
	ID           uuid.UUID  `json:"id"`
	Type         string     `json:"type"`
	Suspended    bool       `json:"suspended"`
	State        string     `json:"state"` // new | learning | review | relearning
	DueAt        time.Time  `json:"due_at"`
	Reps         int        `json:"reps"`
	Lapses       int        `json:"lapses"`
	LastReviewAt *time.Time `json:"last_review_at"`
}

// CardID is the card's id: UUIDv5 of the note id and the card type, so
// every device derives the same id offline (SYSTEM_DESIGN §4.3).
func CardID(noteID uuid.UUID, cardType string) uuid.UUID {
	return uuid.NewSHA1(noteID, []byte(cardType))
}

// cardTypesFor picks the cards a new note gets from its deck's card types.
// Word notes can't have cloze cards; sentence notes have recognition and,
// when they carry a cloze text, cloze. A note always gets at least one card.
func cardTypesFor(kind string, f Fields, deckTypes []string) []string {
	var out []string
	for _, t := range deckTypes {
		switch {
		case kind == "word" && t != Cloze:
			out = append(out, t)
		case kind == "sentence" && t == Recognition:
			out = append(out, t)
		case kind == "sentence" && t == Cloze && f.Cloze != "":
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		out = []string{Recognition}
	}
	return out
}

func stateName(s int16) string {
	switch s {
	case scheduling.StateNew:
		return "new"
	case scheduling.StateLearning:
		return "learning"
	case scheduling.StateReview:
		return "review"
	case scheduling.StateRelearning:
		return "relearning"
	}
	return "unknown"
}

// DictEntry is what study needs from a dictionary entry.
type DictEntry struct {
	ID      int64
	Hanzi   string // simplified
	Pinyin  string // tone marks
	Meaning string // definitions joined with "; "
}

// ErrNoEntry is returned by Dictionary.Entry for an unknown id.
var ErrNoEntry = errors.New("no such dictionary entry")

// Dictionary is the part of internal/dictionary that study uses.
type Dictionary interface {
	// Entry returns one entry, or ErrNoEntry.
	Entry(ctx context.Context, id int64) (DictEntry, error)
	// Headword returns the entries written exactly as hanzi (simplified or
	// traditional), best first.
	Headword(ctx context.Context, hanzi string) ([]DictEntry, error)
}

// Devices is the part of internal/identity that study uses.
type Devices interface {
	OwnsDevice(ctx context.Context, userID, deviceID uuid.UUID) (bool, error)
}

// ParamsSource provides a user's FSRS weights (internal/scheduling).
type ParamsSource interface {
	UserParams(ctx context.Context, userID uuid.UUID) (scheduling.Params, error)
}
