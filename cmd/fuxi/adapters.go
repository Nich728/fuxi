package main

import (
	"context"
	"errors"
	"strings"

	"github.com/Nich728/fuxi/internal/dictionary"
	"github.com/Nich728/fuxi/internal/study"
)

// studyDictionary gives internal/study the dictionary lookups it needs
// through study's own small interface (SYSTEM_DESIGN §2, package rule).
type studyDictionary struct{ svc *dictionary.Service }

func (d studyDictionary) Entry(ctx context.Context, id int64) (study.DictEntry, error) {
	e, err := d.svc.Lookup(ctx, id)
	if errors.Is(err, dictionary.ErrNotFound) {
		return study.DictEntry{}, study.ErrNoEntry
	}
	if err != nil {
		return study.DictEntry{}, err
	}
	return toStudyEntry(e), nil
}

func (d studyDictionary) Headword(ctx context.Context, hanzi string) ([]study.DictEntry, error) {
	es, err := d.svc.Headword(ctx, hanzi)
	if err != nil {
		return nil, err
	}
	out := make([]study.DictEntry, len(es))
	for i, e := range es {
		out[i] = toStudyEntry(e)
	}
	return out, nil
}

func toStudyEntry(e dictionary.Entry) study.DictEntry {
	return study.DictEntry{ID: e.ID, Hanzi: e.Simplified, Pinyin: e.Pinyin, Meaning: strings.Join(e.Definitions, "; ")}
}
