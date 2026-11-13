package dictionary

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseCedict(t *testing.T) {
	f, err := os.Open("testdata/cedict_sample.u8")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	entries, err := ParseCedict(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 27 {
		t.Fatalf("got %d entries, want 27 (the duplicate 马 ma3 line is merged)", len(entries))
	}
	byKey := map[string]CedictEntry{}
	for _, e := range entries {
		byKey[e.SourceKey] = e
	}

	nuer := byKey["女兒|女儿|nu:3 er2"]
	if nuer.PinyinMarked != "nǚ'ér" || nuer.PinyinToneless != "nuer" {
		t.Errorf("女儿: marked %q toneless %q", nuer.PinyinMarked, nuer.PinyinToneless)
	}
	ma := byKey["馬|马|ma3"]
	if want := []string{"horse", "a duplicate line whose definitions are merged"}; !reflect.DeepEqual(ma.Definitions, want) {
		t.Errorf("马 definitions = %q, want %q", ma.Definitions, want)
	}
	if _, ok := byKey["馬|马|Ma3"]; !ok {
		t.Error("the surname 马 Ma3 must stay a separate entry")
	}
	for key, want := range map[string]bool{
		"馬|马|Ma3":           true, // surname
		"傌|骂|ma4":           true, // variant of
		"綠|绿|lu4":           true, // used in names
		"中文|中文|Zhong1 wen2": true, // capitalised, so secondary; HSK listing keeps it from being demoted
		"馬|马|ma3":           false,
		"詞典|词典|ci2 dian3":   false, // "also written" as a later sense doesn't count
	} {
		if got := byKey[key].Secondary; got != want {
			t.Errorf("%s secondary = %v, want %v", key, got, want)
		}
	}
}

func TestMatchHSK(t *testing.T) {
	cands := []HSKCandidate{
		{ID: 1, Simplified: "吗", Marked: "ma", Toneless: "ma"},
		{ID: 2, Simplified: "吗", Marked: "má", Toneless: "ma"},
		{ID: 3, Simplified: "一下", Marked: "yīxià", Toneless: "yixia"},
		{ID: 4, Simplified: "骂", Marked: "mà", Toneless: "ma"},
		{ID: 5, Simplified: "骂", Marked: "mà", Toneless: "ma", Secondary: true}, // variant
		{ID: 6, Simplified: "中文", Marked: "Zhōngwén", Toneless: "zhongwen", Secondary: true},
		{ID: 7, Simplified: "了", Marked: "le", Toneless: "le"},
		{ID: 8, Simplified: "了", Marked: "liǎo", Toneless: "liao"},
		{ID: 9, Simplified: "玩儿", Marked: "wánr", Toneless: "wanr"},
	}
	words := []HSKWord{
		{Word: "吗", Level: 1, Marked: "ma", Toneless: "ma"},
		{Word: "一下", Level: 1, Marked: "yíxià", Toneless: "yixia"}, // tone sandhi
		{Word: "骂", Level: 5, Marked: "mà", Toneless: "ma"},
		{Word: "中文", Level: 1, Marked: "zhōngwén", Toneless: "zhongwen"},
		{Word: "了", Level: 1, Marked: "le", Toneless: "le"},
		{Word: "了", Level: 5, Marked: "liǎo", Toneless: "liao"},
		{Word: "玩儿", Level: 2, Marked: "wán", Toneless: "wan"}, // nothing matches: whole-word fallback
		{Word: "没有词", Level: 2, Marked: "méiyǒucí", Toneless: "meiyouci"},
	}
	tags, unmatched := MatchHSK(cands, words)
	want := map[int64]int{1: 1, 3: 1, 4: 5, 6: 1, 7: 1, 8: 5, 9: 2}
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("tags = %v, want %v", tags, want)
	}
	if !reflect.DeepEqual(unmatched, []string{"没有词"}) {
		t.Errorf("unmatched = %v", unmatched)
	}
}

func TestParseCedictRejectsMalformed(t *testing.T) {
	for _, line := range []string{
		"妈 [ma1] /mom/", // missing simplified
		"媽 妈 ma1 /mom/", // no brackets
		"媽 妈 [ma1] //",  // no definitions
	} {
		if _, err := ParseCedict(strings.NewReader(line)); err == nil {
			t.Errorf("ParseCedict(%q) = nil error", line)
		}
	}
}

func TestParseHSK(t *testing.T) {
	f, err := os.Open("testdata/hsk_sample.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	words, err := ParseHSK(f)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]HSKWord{}
	for _, w := range words {
		got[w.Word+"/"+w.Toneless] = w
	}
	tests := []struct {
		key   string
		level int
	}{
		{"了/le", 1},
		{"了/liao", 5}, // "了2": sense number dropped, its own reading kept
		{"骂/ma", 7},   // levels 7–9 share one band
		{"女儿/nuer", 2},
		{"喜欢/xihuan", 1},
	}
	for _, tt := range tests {
		w, ok := got[tt.key]
		if !ok {
			t.Errorf("%s missing", tt.key)
			continue
		}
		if w.Level != tt.level {
			t.Errorf("%s level = %d, want %d", tt.key, w.Level, tt.level)
		}
	}
}

func TestParseHSKKeepsLowestLevel(t *testing.T) {
	in := "word\tlevel\talso\tpinyin\tpos\n一下\t4\t\tyíxià\t\n一下\t1\t\tyíxià\t\n"
	words, err := ParseHSK(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 1 || words[0].Level != 1 {
		t.Fatalf("got %+v, want one word at level 1", words)
	}
}

func TestTatoebaJoin(t *testing.T) {
	links, err := ReadTatoebaLinks(strings.NewReader("1\t30\n1\t20\n2\t40\n"))
	if err != nil {
		t.Fatal(err)
	}
	cmn, err := ReadTatoebaSentences(strings.NewReader("2\tcmn\t他不知道。\n1\tcmn\t我喜欢中文。\n3\tcmn\t没有翻译。\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := ReadTatoebaSentences(strings.NewReader("20\teng\tI like Chinese.\n30\teng\tI love Chinese.\n99\teng\tUnused.\n"),
		map[string]bool{"20": true, "30": true, "40": true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := eng["99"]; ok {
		t.Error("unlinked English sentence should be filtered out")
	}
	got := JoinTatoeba(cmn, links, eng)
	want := []TatoebaSentence{
		{ID: "1", ZH: "我喜欢中文。", EN: "I like Chinese."}, // lowest linked id wins
		{ID: "2", ZH: "他不知道。"},                         // linked translation 40 isn't in the file
		{ID: "3", ZH: "没有翻译。"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JoinTatoeba =\n%+v\nwant\n%+v", got, want)
	}
}

func TestSegmenter(t *testing.T) {
	seg := NewSegmenter([]SegmentEntry{
		{ID: 1, Simplified: "我", Traditional: "我", HSKLevel: 1},
		{ID: 2, Simplified: "喜欢", Traditional: "喜歡", HSKLevel: 1},
		{ID: 3, Simplified: "中文", Traditional: "中文", HSKLevel: 1, Secondary: true},
		{ID: 4, Simplified: "中", Traditional: "中", HSKLevel: 1},
		{ID: 5, Simplified: "了", Traditional: "了", HSKLevel: 5}, // liǎo
		{ID: 6, Simplified: "了", Traditional: "了", HSKLevel: 1}, // le: lower level wins
		{ID: 7, Simplified: "文", Traditional: "文"},
		{ID: 8, Simplified: "绿", Traditional: "綠", Secondary: true}, // "used in names"
		{ID: 9, Simplified: "绿", Traditional: "綠"},                  // "green"
	})
	if got := seg.Segment("绿"); !reflect.DeepEqual(got, []int64{9}) {
		t.Errorf("Segment(绿) = %v, want the ordinary entry 9", got)
	}
	tests := []struct {
		text string
		want []int64
	}{
		{"我喜欢中文了。", []int64{1, 2, 3, 6}}, // longest match: 中文, not 中 + 文
		{"我喜歡中文", []int64{1, 2, 3}},      // traditional forms match too
		{"我我我", []int64{1}},              // each entry once
		{"Tom 喜欢 X 中", []int64{2, 4}},    // Latin text and spaces skipped
		{"鑫", nil},                       // unknown character
	}
	for _, tt := range tests {
		if got := seg.Segment(tt.text); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Segment(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}
