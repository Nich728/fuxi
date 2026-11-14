package dictionary

import "unicode"

// Segmenter splits Chinese text into dictionary words by greedy longest match
// (SYSTEM_DESIGN §5 step 4). It matches both simplified and traditional
// forms, because Tatoeba has sentences in each.
type Segmenter struct {
	words  map[string]int64 // headword → chosen entry id
	maxLen int              // longest headword, in runes
}

// SegmentEntry is the dictionary data the segmenter needs about one entry.
type SegmentEntry struct {
	ID          int64
	Simplified  string
	Traditional string
	HSKLevel    int  // 0 if none
	Secondary   bool // a name or pointer entry (see IsSecondary)
}

// NewSegmenter builds a segmenter. When several entries share a headword
// (了 le / liǎo, 绿 "green" / 绿 "used in names"), one is chosen
// deterministically: a simplified match over a traditional one, then any
// entry over a name or pointer that isn't on the HSK list, then the lowest
// HSK level, then the lowest id.
func NewSegmenter(entries []SegmentEntry) *Segmenter {
	type cand struct {
		e          SegmentEntry
		simplified bool
	}
	demoted := func(c cand) bool { return c.e.Secondary && c.e.HSKLevel == 0 }
	better := func(a, b cand) bool {
		if a.simplified != b.simplified {
			return a.simplified
		}
		if demoted(a) != demoted(b) {
			return !demoted(a)
		}
		al, bl := levelOrMax(a.e.HSKLevel), levelOrMax(b.e.HSKLevel)
		if al != bl {
			return al < bl
		}
		return a.e.ID < b.e.ID
	}
	best := map[string]cand{}
	consider := func(word string, c cand) {
		if word == "" {
			return
		}
		if cur, ok := best[word]; !ok || better(c, cur) {
			best[word] = c
		}
	}
	for _, e := range entries {
		consider(e.Simplified, cand{e, true})
		consider(e.Traditional, cand{e, false})
	}
	s := &Segmenter{words: make(map[string]int64, len(best))}
	for w, c := range best {
		s.words[w] = c.e.ID
		if n := len([]rune(w)); n > s.maxLen {
			s.maxLen = n
		}
	}
	return s
}

func levelOrMax(l int) int {
	if l == 0 {
		return 99
	}
	return l
}

// Segment returns the entry ids of the dictionary words found in text, in
// order of first appearance, each once. Runs of non-Han characters are skipped.
func (s *Segmenter) Segment(text string) []int64 {
	runes := []rune(text)
	seen := map[int64]bool{}
	var ids []int64
	for i := 0; i < len(runes); {
		if !unicode.Is(unicode.Han, runes[i]) {
			i++
			continue
		}
		matched := 0
		for n := min(s.maxLen, len(runes)-i); n >= 1; n-- {
			if id, ok := s.words[string(runes[i:i+n])]; ok {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
				matched = n
				break
			}
		}
		if matched == 0 {
			matched = 1 // a character the dictionary doesn't know
		}
		i += matched
	}
	return ids
}
