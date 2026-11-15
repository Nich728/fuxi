package study

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDayBounds(t *testing.T) {
	jakarta, _ := time.LoadLocation("Asia/Jakarta") // UTC+7, no DST
	ny, _ := time.LoadLocation("America/New_York")
	utc := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	tests := []struct {
		name       string
		now        string
		loc        *time.Location
		hour       int
		start, end string
	}{
		{"jakarta afternoon", "2026-11-14T08:00:00Z", jakarta, 4, "2026-11-13T21:00:00Z", "2026-11-14T21:00:00Z"},
		// 01:30 local on the 15th still belongs to the 14th (PRD R5).
		{"jakarta after midnight", "2026-11-14T18:30:00Z", jakarta, 4, "2026-11-13T21:00:00Z", "2026-11-14T21:00:00Z"},
		{"jakarta exactly 4am", "2026-11-14T21:00:00Z", jakarta, 4, "2026-11-14T21:00:00Z", "2026-11-15T21:00:00Z"},
		{"midnight start", "2026-11-14T16:59:59Z", jakarta, 0, "2026-11-13T17:00:00Z", "2026-11-14T17:00:00Z"},
		// Clocks go forward at 2 am on 8 March 2026 in New York, so the study
		// day running from 4 am on the 7th is 23 hours long.
		{"spring forward", "2026-03-08T05:00:00Z", ny, 4, "2026-03-07T09:00:00Z", "2026-03-08T08:00:00Z"},
		{"day after spring forward", "2026-03-08T12:00:00Z", ny, 4, "2026-03-08T08:00:00Z", "2026-03-09T08:00:00Z"},
		// Clocks go back at 2 am on 1 November 2026: that day is 25 hours.
		{"fall back", "2026-11-01T05:00:00Z", ny, 4, "2026-10-31T08:00:00Z", "2026-11-01T09:00:00Z"},
		// 2 am doesn't exist on 8 March; Go resolves it to 06:00 UTC, and the
		// day before ends at the same instant, so days stay contiguous.
		{"start hour skipped", "2026-03-08T12:00:00Z", ny, 2, "2026-03-08T06:00:00Z", "2026-03-09T06:00:00Z"},
		{"day before skipped start", "2026-03-08T05:00:00Z", ny, 2, "2026-03-07T07:00:00Z", "2026-03-08T06:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := DayBounds(utc(tt.now), tt.loc, tt.hour)
			if !start.Equal(utc(tt.start)) || !end.Equal(utc(tt.end)) {
				t.Errorf("got [%v, %v), want [%s, %s)", start, end, tt.start, tt.end)
			}
			now := utc(tt.now)
			if now.Before(start) || !now.Before(end) {
				t.Errorf("now %v not inside its own day", now)
			}
		})
	}
}

func TestDeckSettings(t *testing.T) {
	i := func(v int) *int { return &v }
	f := func(v float64) *float64 { return &v }
	tests := []struct {
		name  string
		patch *DeckSettingsPatch
		ok    bool
	}{
		{"defaults", nil, true},
		{"all types", &DeckSettingsPatch{CardTypes: []string{Recognition, Recall, Tones, Cloze}}, true},
		{"zero new per day", &DeckSettingsPatch{NewPerDay: i(0)}, true},
		{"negative new per day", &DeckSettingsPatch{NewPerDay: i(-1)}, false},
		{"empty types", &DeckSettingsPatch{CardTypes: []string{}}, false},
		{"unknown type", &DeckSettingsPatch{CardTypes: []string{"audio"}}, false},
		{"duplicate type", &DeckSettingsPatch{CardTypes: []string{Recall, Recall}}, false},
		{"retention 0.80", &DeckSettingsPatch{DesiredRetention: f(0.80)}, true},
		{"retention 0.97", &DeckSettingsPatch{DesiredRetention: f(0.97)}, true},
		{"retention 0.99", &DeckSettingsPatch{DesiredRetention: f(0.99)}, false},
		{"retention 0.5", &DeckSettingsPatch{DesiredRetention: f(0.5)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DefaultDeckSettings().apply(tt.patch)
			if (err == nil) != tt.ok {
				t.Errorf("err = %v, want ok=%v", err, tt.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Errorf("err %v is not ErrInvalid", err)
			}
		})
	}
}

func TestCardTypesFor(t *testing.T) {
	all := []string{Recognition, Recall, Tones, Cloze}
	tests := []struct {
		kind  string
		f     Fields
		deck  []string
		types []string
	}{
		{"word", Fields{}, all, []string{Recognition, Recall, Tones}},
		{"word", Fields{}, []string{Cloze}, []string{Recognition}},
		{"sentence", Fields{}, all, []string{Recognition}},
		{"sentence", Fields{Cloze: "我{{c1::喜欢}}你"}, all, []string{Recognition, Cloze}},
		{"sentence", Fields{Cloze: "x"}, []string{Recall}, []string{Recognition}},
	}
	for _, tt := range tests {
		if got := cardTypesFor(tt.kind, tt.f, tt.deck); !slices.Equal(got, tt.types) {
			t.Errorf("%s %v %v = %v, want %v", tt.kind, tt.f, tt.deck, got, tt.types)
		}
	}
}

func TestCardIDIsDeterministic(t *testing.T) {
	note := uuid.MustParse("01900000-0000-7000-8000-000000000001")
	// Another device computes the same id independently.
	if a, b := CardID(note, Recall), CardID(uuid.MustParse(note.String()), Recall); a != b {
		t.Error("same input, different ids")
	}
	if CardID(note, Recall) == CardID(note, Recognition) {
		t.Error("card types share an id")
	}
	if v := CardID(note, Recall).Version(); v != 5 {
		t.Errorf("version = %d, want 5", v)
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []CSVRow
		wantErr string
	}{
		{
			name: "header, BOM, numbered pinyin and blank line",
			in:   "\xEF\xBB\xBFhanzi,pinyin,meaning\n你好,ni3 hao3,hello\n\n学习,xuéxí,\"to study, to learn\",lesson 2,hsk1 class\n",
			want: []CSVRow{
				{Line: 2, Hanzi: "你好", Pinyin: "ni3 hao3", Meaning: "hello"},
				{Line: 4, Hanzi: "学习", Pinyin: "xuéxí", Meaning: "to study, to learn", Notes: "lesson 2", Tags: []string{"hsk1", "class"}},
			},
		},
		{
			name: "tab separated without header",
			in:   "猫\tmāo\tcat\n狗\tgǒu\tdog",
			want: []CSVRow{{Line: 1, Hanzi: "猫", Pinyin: "māo", Meaning: "cat"}, {Line: 2, Hanzi: "狗", Pinyin: "gǒu", Meaning: "dog"}},
		},
		{name: "hanzi only", in: "电脑\n", want: []CSVRow{{Line: 1, Hanzi: "电脑"}}},
		{name: "empty", in: "\n\n", wantErr: "no rows"},
		{name: "header only", in: "汉字,pinyin,meaning\n", wantErr: "no rows"},
		{name: "not utf-8", in: "\xff\xfe\x00a", wantErr: "UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCSV([]byte(tt.in))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d rows %+v, want %d", len(got), got, len(tt.want))
			}
			for i := range got {
				g, w := got[i], tt.want[i]
				if g.Line != w.Line || g.Hanzi != w.Hanzi || g.Pinyin != w.Pinyin || g.Meaning != w.Meaning ||
					g.Notes != w.Notes || !slices.Equal(g.Tags, w.Tags) {
					t.Errorf("row %d = %+v, want %+v", i, g, w)
				}
			}
		})
	}
}

func TestParseCSVLimits(t *testing.T) {
	var b strings.Builder
	for range MaxCSVRows + 1 {
		b.WriteString("一,yī,one\n")
	}
	if _, err := ParseCSV([]byte(b.String())); err == nil || !strings.Contains(err.Error(), "5000") {
		t.Errorf("5001 rows: err = %v", err)
	}
	if _, err := ParseCSV(make([]byte, MaxCSVBytes+1)); err == nil {
		t.Error("oversized file accepted")
	}
}

func TestNormalizePinyin(t *testing.T) {
	tests := map[string]string{
		"ni3 hao3": "nǐhǎo",
		"ni3hao3":  "nǐhǎo",
		"lv4":      "lǜ",
		"lü4":      "lǜ",
		"xi1an1":   "xī'ān",
		"nǐ hǎo":   "nǐ hǎo", // already marked: kept as written
		"":         "",
	}
	for in, want := range tests {
		if got := normalizePinyin(in); got != want {
			t.Errorf("normalizePinyin(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRowNoteID(t *testing.T) {
	imp := uuid.MustParse("01900000-0000-7000-8000-0000000000aa")
	started := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	a, b := rowNoteID(imp, started, 2), rowNoteID(imp, started, 3)
	if a != rowNoteID(imp, started, 2) {
		t.Error("not deterministic")
	}
	if a.Version() != 7 || a.Variant() != uuid.RFC4122 {
		t.Errorf("version %d variant %v", a.Version(), a.Variant())
	}
	if a.String() >= b.String() {
		t.Errorf("line 2 id %s doesn't sort before line 3 id %s", a, b)
	}
	other := uuid.MustParse("01900000-0000-7000-8000-0000000000bb")
	if rowNoteID(other, started, 2) == a {
		t.Error("different imports share an id")
	}
}

func TestHLC(t *testing.T) {
	clk := &stepClock{t: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)}
	h := newHLC(clk)
	a, b := h.next(), h.next()
	if a >= b {
		t.Errorf("%s not before %s", a, b)
	}
	clk.t = clk.t.Add(-time.Second) // the clock went backwards
	if c := h.next(); c <= b {
		t.Errorf("%s not after %s after the clock went back", c, b)
	}
	if len(a) != len("0000000000000:00000:server") {
		t.Errorf("not fixed width: %q", a)
	}
}

type stepClock struct{ t time.Time }

func (c *stepClock) Now() time.Time { return c.t }
