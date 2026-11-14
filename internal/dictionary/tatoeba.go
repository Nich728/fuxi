package dictionary

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// TatoebaSentence is a Mandarin sentence with one English translation, if any.
type TatoebaSentence struct {
	ID string // Tatoeba sentence id
	ZH string
	EN string
}

// ReadTatoebaLinks reads "cmn-eng_links.tsv" (cmn_id <TAB> eng_id) and
// returns, for each Mandarin sentence, its linked English ids in ascending order.
func ReadTatoebaLinks(r io.Reader) (map[string][]string, error) {
	links := map[string][]string{}
	err := eachTSV(r, 2, func(cols []string) {
		links[cols[0]] = append(links[cols[0]], cols[1])
	})
	for _, ids := range links {
		sort.Slice(ids, func(i, j int) bool { return lessNumeric(ids[i], ids[j]) })
	}
	return links, err
}

// ReadTatoebaSentences reads "<lang>_sentences.tsv" (id <TAB> lang <TAB> text).
// When keep is non-nil, only ids in it are returned, which keeps the large
// English file small in memory.
func ReadTatoebaSentences(r io.Reader, keep map[string]bool) (map[string]string, error) {
	out := map[string]string{}
	err := eachTSV(r, 3, func(cols []string) {
		if keep == nil || keep[cols[0]] {
			out[cols[0]] = strings.TrimSpace(cols[2])
		}
	})
	return out, err
}

// JoinTatoeba pairs each Mandarin sentence with its first-linked English
// translation (lowest id: the oldest, usually the most reviewed). Output is
// sorted by Tatoeba id so imports are deterministic.
func JoinTatoeba(cmn map[string]string, links map[string][]string, eng map[string]string) []TatoebaSentence {
	out := make([]TatoebaSentence, 0, len(cmn))
	for id, zh := range cmn {
		s := TatoebaSentence{ID: id, ZH: zh}
		for _, engID := range links[id] {
			if en, ok := eng[engID]; ok {
				s.EN = en
				break
			}
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return lessNumeric(out[i].ID, out[j].ID) })
	return out
}

func eachTSV(r io.Reader, minCols int, fn func([]string)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" {
			continue
		}
		cols := strings.SplitN(line, "\t", minCols)
		if len(cols) < minCols {
			return fmt.Errorf("line %d: want %d tab-separated columns", lineNo, minCols)
		}
		fn(cols)
	}
	return sc.Err()
}

// lessNumeric orders decimal id strings numerically.
func lessNumeric(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}
