package dictionary

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/Nich728/fuxi/internal/dictionary/pinyin"
)

// HSKWord is one reading of a word in the HSK 3.0 examination syllabus.
type HSKWord struct {
	Word     string // simplified
	Level    int    // 1–6, or 7 for the combined 7–9 band (ADR-0016)
	Marked   string // syllabus pinyin, tone marks kept: "zhīdào"
	Toneless string // syllabus pinyin without tones: "zhidao"
}

// ParseHSK reads the 2025 examination syllabus word table
// (harukicoder/hsk30 corpus/syllabus2025/syllabus2025_words.tsv):
//
//	word	level	also	pinyin	pos
//
// Homographs graded twice are written with a sense number ("了2"); the number
// is dropped and the reading kept. A reading listed at several levels keeps
// the lowest (SYSTEM_DESIGN §5 step 5).
func ParseHSK(r io.Reader) ([]HSKWord, error) {
	sc := bufio.NewScanner(r)
	type key struct{ word, marked string }
	best := map[key]HSKWord{}
	var order []key
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), "\r")
		if lineNo == 1 && strings.HasPrefix(line, "word\t") {
			continue // header
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 4 {
			return nil, fmt.Errorf("hsk line %d: want at least 4 columns, got %d", lineNo, len(cols))
		}
		word := strings.TrimRight(strings.TrimSpace(cols[0]), "0123456789")
		level, err := strconv.Atoi(strings.TrimSpace(cols[1]))
		if err != nil || level < 1 || level > 9 {
			return nil, fmt.Errorf("hsk line %d: bad level %q", lineNo, cols[1])
		}
		if level > 7 {
			level = 7 // levels 7–9 share one band
		}
		if word == "" {
			return nil, fmt.Errorf("hsk line %d: empty word", lineNo)
		}
		w := HSKWord{
			Word:     word,
			Level:    level,
			Marked:   pinyin.NormalizeMarked(cols[3]),
			Toneless: pinyin.NormalizeToneless(cols[3]),
		}
		k := key{w.Word, w.Marked}
		prev, seen := best[k]
		if !seen {
			order = append(order, k)
		}
		if !seen || level < prev.Level {
			best[k] = w
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read hsk: %w", err)
	}
	out := make([]HSKWord, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out, nil
}

// HSKCandidate is the dictionary data needed to place an HSK word.
type HSKCandidate struct {
	ID         int64
	Simplified string
	Marked     string // dict_entries.pinyin_marked
	Toneless   string // dict_entries.pinyin_toneless
	Secondary  bool
}

// MatchHSK decides which dictionary entries each syllabus word tags.
// For each listed reading, in order of preference:
//
//  1. ordinary entries with the same tone-marked pinyin (吗 ma, not má or mǎ);
//  2. ordinary entries with the same toneless pinyin, for readings the
//     syllabus writes with tone sandhi (一下 yíxià vs CC-CEDICT yi1 xia4);
//
// and if no reading of a word matched anything, every ordinary entry for the
// word, or failing that every entry (some words exist only as variants).
// An entry tagged by several readings keeps the lowest level. Words with no
// entry at all are returned as unmatched, sorted.
func MatchHSK(cands []HSKCandidate, words []HSKWord) (map[int64]int, []string) {
	byWord := map[string][]HSKCandidate{}
	for _, c := range cands {
		byWord[c.Simplified] = append(byWord[c.Simplified], c)
	}
	readings := map[string][]HSKWord{}
	var wordOrder []string
	for _, w := range words {
		if _, ok := readings[w.Word]; !ok {
			wordOrder = append(wordOrder, w.Word)
		}
		readings[w.Word] = append(readings[w.Word], w)
	}

	tags := map[int64]int{}
	tag := func(id int64, level int) {
		if cur, ok := tags[id]; !ok || level < cur {
			tags[id] = level
		}
	}
	var unmatched []string
	for _, word := range wordOrder {
		cs := byWord[word]
		if len(cs) == 0 {
			unmatched = append(unmatched, word)
			continue
		}
		any := false
		minLevel := 99
		for _, r := range readings[word] {
			minLevel = min(minLevel, r.Level)
			hits := filter(cs, func(c HSKCandidate) bool {
				return !c.Secondary && pinyin.NormalizeMarked(c.Marked) == r.Marked
			})
			if len(hits) == 0 {
				hits = filter(cs, func(c HSKCandidate) bool { return !c.Secondary && c.Toneless == r.Toneless })
			}
			for _, c := range hits {
				tag(c.ID, r.Level)
				any = true
			}
		}
		if any {
			continue
		}
		fallback := filter(cs, func(c HSKCandidate) bool { return !c.Secondary })
		if len(fallback) == 0 {
			fallback = cs
		}
		for _, c := range fallback {
			tag(c.ID, minLevel)
		}
	}
	sort.Strings(unmatched)
	return tags, unmatched
}

func filter(cs []HSKCandidate, keep func(HSKCandidate) bool) []HSKCandidate {
	var out []HSKCandidate
	for _, c := range cs {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}
