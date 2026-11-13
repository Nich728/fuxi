package pinyin

import "testing"

func TestToMarked(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ci2 dian3", "cídiǎn"},
		{"chuan2", "chuán"},
		{"lu:4", "lǜ"},
		{"nu:3 er2", "nǚ'ér"},
		{"xi1 an1", "xī'ān"},
		{"Xi1 an1", "Xī'ān"},
		{"Bei3 jing1", "Běijīng"},
		{"ma5", "ma"},
		{"ma", "ma"},
		{"zhi1 dao5", "zhīdao"},
		{"dou1", "dōu"}, // o in "ou"
		{"gui4", "guì"}, // last vowel
		{"liu2", "liú"}, // last vowel
		{"hao3", "hǎo"}, // a wins
		{"xue2", "xué"}, // e wins
		{"nar4", "nàr"}, // erhua spelled into the syllable
		{"wan2 r5", "wánr"},
		{"m2", "m"},             // vowel-less interjection
		{"A A zhi4", "A A zhì"}, // Latin letters pass through
		{"ka3 la1 O K", "kǎlā O K"},
		{"yi1 , er4", "yī , èr"},
		{"Lu:4 Xun4", "LǜXùn"}, // case is kept as CC-CEDICT writes it
	}
	for _, tt := range tests {
		if got := ToMarked(tt.in); got != tt.want {
			t.Errorf("ToMarked(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToToneless(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ci2 dian3", "cidian"},
		{"Lu:4 xing2", "luxing"},
		{"ma1", "ma"},
		{"A A zhi4", "zhi"},
		{"wan2 r5", "wanr"},
	}
	for _, tt := range tests {
		if got := ToToneless(tt.in); got != tt.want {
			t.Errorf("ToToneless(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	numbered := []struct{ in, want string }{
		{"Ci2 Dian3", "ci2dian3"},
		{"lv4", "lu:4"},
		{"lü4", "lu:4"},
		{"lu:4", "lu:4"},
	}
	for _, tt := range numbered {
		if got := NormalizeNumbered(tt.in); got != tt.want {
			t.Errorf("NormalizeNumbered(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	toneless := []struct{ in, want string }{
		{"Ci Dian", "cidian"},
		{"cídiǎn", "cidian"},
		{"lǜ", "lu"},
		{"lv", "lu"},
		{"xi'an", "xian"},
	}
	for _, tt := range toneless {
		if got := NormalizeToneless(tt.in); got != tt.want {
			t.Errorf("NormalizeToneless(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
