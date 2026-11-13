// Package pinyin converts CC-CEDICT numbered pinyin ("ci2 dian3", "lu:4")
// into tone-marked ("cídiǎn", "lǜ") and toneless ("cidian", "lu") forms
// (SYSTEM_DESIGN §5 step 2).
package pinyin

import (
	"strings"
	"unicode"
)

// marks[v][tone-1] is vowel v carrying tones 1–4.
var marks = map[rune][4]rune{
	'a': {'ā', 'á', 'ǎ', 'à'},
	'e': {'ē', 'é', 'ě', 'è'},
	'i': {'ī', 'í', 'ǐ', 'ì'},
	'o': {'ō', 'ó', 'ǒ', 'ò'},
	'u': {'ū', 'ú', 'ǔ', 'ù'},
	'ü': {'ǖ', 'ǘ', 'ǚ', 'ǜ'},
	'A': {'Ā', 'Á', 'Ǎ', 'À'},
	'E': {'Ē', 'É', 'Ě', 'È'},
	'I': {'Ī', 'Í', 'Ǐ', 'Ì'},
	'O': {'Ō', 'Ó', 'Ǒ', 'Ò'},
	'U': {'Ū', 'Ú', 'Ǔ', 'Ù'},
	'Ü': {'Ǖ', 'Ǘ', 'Ǚ', 'Ǜ'},
}

// Syllable is one parsed numbered-pinyin syllable.
type Syllable struct {
	Letters string // with ü for u: / v, original case kept
	Tone    int    // 1–4, or 5 for neutral / unmarked
}

// ParseSyllable parses a numbered syllable such as "lu:4", "Bei3", "ma5" or "r5".
// It reports false for tokens that aren't pinyin (punctuation, Latin letters
// like the "A" in "A A zhi4", "%", "·").
func ParseSyllable(tok string) (Syllable, bool) {
	if tok == "" {
		return Syllable{}, false
	}
	tone := 5
	body := tok
	if last := tok[len(tok)-1]; last >= '1' && last <= '5' {
		tone = int(last - '0')
		body = tok[:len(tok)-1]
	}
	body = strings.NewReplacer("u:", "ü", "U:", "Ü", "v", "ü", "V", "Ü").Replace(body)
	if body == "" {
		return Syllable{}, false
	}
	hasVowel := false
	for _, r := range body {
		if !unicode.IsLetter(r) {
			return Syllable{}, false
		}
		if isVowel(r) {
			hasVowel = true
		}
	}
	// "r5" (erhua) and "m2"/"n2"/"ng4" interjections are syllables without vowels.
	lower := strings.ToLower(body)
	if !hasVowel && lower != "r" && lower != "m" && lower != "n" && lower != "ng" && lower != "hm" && lower != "hng" {
		return Syllable{}, false
	}
	// A single capital letter with no tone digit is a Latin letter, not pinyin ("A A zhi4").
	if tone == 5 && len(body) == 1 && unicode.IsUpper(rune(body[0])) {
		return Syllable{}, false
	}
	return Syllable{Letters: body, Tone: tone}, true
}

// Marked returns the syllable with its tone mark, e.g. "lǜ", "Běi", "ma".
func (s Syllable) Marked() string {
	if s.Tone < 1 || s.Tone > 4 {
		return s.Letters
	}
	runes := []rune(s.Letters)
	i := markIndex(runes)
	if i < 0 {
		return s.Letters // vowel-less syllables such as "m2" keep their letters
	}
	runes[i] = marks[runes[i]][s.Tone-1]
	return string(runes)
}

// markIndex finds the vowel that carries the tone mark:
// a or e if present; o in "ou"; otherwise the last vowel.
func markIndex(r []rune) int {
	last := -1
	for i, c := range r {
		lc := unicode.ToLower(c)
		if lc == 'a' || lc == 'e' {
			return i
		}
		if lc == 'o' && i+1 < len(r) && unicode.ToLower(r[i+1]) == 'u' {
			return i
		}
		if isVowel(c) {
			last = i
		}
	}
	return last
}

func isVowel(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u', 'ü':
		return true
	}
	return false
}

// ToMarked converts a whole CC-CEDICT pinyin field to tone marks.
// Syllables of one word are joined without spaces, with an apostrophe before
// a syllable starting with a, o or e ("xi1 an1" → "xī'ān"). Tokens that aren't
// pinyin are kept and separated by spaces ("A A zhi4" → "A A zhì").
func ToMarked(numbered string) string {
	var b strings.Builder
	prevSyllable := false
	for i, tok := range strings.Fields(numbered) {
		s, ok := ParseSyllable(tok)
		switch {
		case !ok:
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(tok)
		case prevSyllable:
			if startsWithAOE(s.Letters) {
				b.WriteByte('\'')
			}
			b.WriteString(s.Marked())
		default:
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(s.Marked())
		}
		prevSyllable = ok
	}
	return b.String()
}

func startsWithAOE(s string) bool {
	switch unicode.ToLower([]rune(s)[0]) {
	case 'a', 'o', 'e':
		return true
	}
	return false
}

// ToToneless converts a CC-CEDICT pinyin field to the search form:
// lowercase letters only, ü written as u ("Lu:4 xing2" → "luxing").
func ToToneless(numbered string) string {
	var b strings.Builder
	for _, tok := range strings.Fields(numbered) {
		s, ok := ParseSyllable(tok)
		if !ok {
			continue
		}
		for _, r := range strings.ToLower(s.Letters) {
			if r == 'ü' {
				r = 'u'
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NormalizeNumbered turns user input such as "Ci2 Dian3", "lv4" or "lü4"
// into the key matched against lower(replace(pinyin_numbered, ' ', ”)): "ci2dian3", "lu:4".
func NormalizeNumbered(q string) string {
	q = strings.ToLower(strings.Join(strings.Fields(q), ""))
	return strings.NewReplacer("u:", "u:", "ü", "u:", "v", "u:").Replace(q)
}

// NormalizeToneless turns user input such as "Ci Dian" or "lǜ" into the
// toneless search key: "cidian", "lu". Tone marks are stripped.
func NormalizeToneless(q string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(q) {
		if base, ok := unmark[r]; ok {
			r = base
		}
		if r == 'ü' || r == 'v' {
			r = 'u'
		}
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NormalizeMarked keeps tone marks but drops case, spaces, apostrophes and
// hyphens, so "Yìsī-bùgǒu" and "yìsībùgǒu" compare equal.
func NormalizeMarked(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// unmark maps each lowercase tone-marked vowel to its base letter.
var unmark = func() map[rune]rune {
	m := map[rune]rune{}
	for v, tones := range marks {
		if unicode.IsLower(v) {
			for _, t := range tones {
				m[t] = v
			}
		}
	}
	return m
}()
