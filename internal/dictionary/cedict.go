package dictionary

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/Nich728/fuxi/internal/dictionary/pinyin"
)

// CedictEntry is one parsed CC-CEDICT line, with the derived pinyin forms.
type CedictEntry struct {
	SourceKey      string // "trad|simp|pinyin", stable across imports
	Traditional    string
	Simplified     string
	PinyinNumbered string
	PinyinMarked   string
	PinyinToneless string
	Definitions    []string
	// Secondary marks names and pointer entries; see IsSecondary.
	Secondary bool
}

// pointerDef matches a first definition that only points elsewhere.
var pointerDef = regexp.MustCompile(`^(\([^)]*\) )?((old|archaic|ancient|erroneous|obsolete|japanese|korean|non-standard|taiwan|dialectal) )*(variant of|used in|see |also written|abbr\. for|surname |name of )`)

// IsSecondary reports whether an entry is a proper noun (CC-CEDICT
// capitalises its pinyin) or a pointer whose main sense is defined elsewhere.
func IsSecondary(pinyinNumbered string, defs []string) bool {
	if r := []rune(pinyinNumbered); len(r) > 0 && unicode.IsUpper(r[0]) {
		return true
	}
	return len(defs) > 0 && pointerDef.MatchString(strings.ToLower(defs[0]))
}

// ParseCedict reads CC-CEDICT text ("Trad Simp [pin1 yin1] /def 1/def 2/")
// and returns its entries in file order. Comment lines start with '#'.
// Lines that share a source key are merged, keeping every definition once,
// so the upsert never touches a row twice.
func ParseCedict(r io.Reader) ([]CedictEntry, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var out []CedictEntry
	index := map[string]int{}
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		e, err := parseCedictLine(line)
		if err != nil {
			return nil, fmt.Errorf("cedict line %d: %w", lineNo, err)
		}
		if i, ok := index[e.SourceKey]; ok {
			out[i].Definitions = appendUnique(out[i].Definitions, e.Definitions...)
			out[i].Secondary = IsSecondary(out[i].PinyinNumbered, out[i].Definitions)
			continue
		}
		index[e.SourceKey] = len(out)
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read cedict: %w", err)
	}
	return out, nil
}

func parseCedictLine(line string) (CedictEntry, error) {
	open := strings.IndexByte(line, '[')
	close := strings.IndexByte(line, ']')
	slash := strings.IndexByte(line, '/')
	if open < 0 || close < open || slash < close {
		return CedictEntry{}, fmt.Errorf("malformed: %q", line)
	}
	heads := strings.Fields(line[:open])
	if len(heads) != 2 {
		return CedictEntry{}, fmt.Errorf("want traditional and simplified, got %q", line[:open])
	}
	py := strings.Join(strings.Fields(line[open+1:close]), " ")
	var defs []string
	for _, d := range strings.Split(strings.Trim(line[slash:], "/"), "/") {
		if d = strings.TrimSpace(d); d != "" {
			defs = appendUnique(defs, d)
		}
	}
	if len(defs) == 0 {
		return CedictEntry{}, fmt.Errorf("no definitions: %q", line)
	}
	return CedictEntry{
		SourceKey:      heads[0] + "|" + heads[1] + "|" + py,
		Traditional:    heads[0],
		Simplified:     heads[1],
		PinyinNumbered: py,
		PinyinMarked:   pinyin.ToMarked(py),
		PinyinToneless: pinyin.ToToneless(py),
		Definitions:    defs,
		Secondary:      IsSecondary(py, defs),
	}, nil
}

func appendUnique(dst []string, items ...string) []string {
outer:
	for _, it := range items {
		for _, d := range dst {
			if d == it {
				continue outer
			}
		}
		dst = append(dst, it)
	}
	return dst
}
