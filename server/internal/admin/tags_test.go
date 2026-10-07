package admin

import "testing"

func TestSanitizeTags(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		err  bool
	}{
		{"normal", "付费用户,羊毛党", "付费用户,羊毛党", false},
		{"trim and drop empty", " 付费用户 ,, 羊毛党 ,", "付费用户,羊毛党", false},
		{"dedupe keeps first", "a,b,a", "a,b", false},
		{"empty clears all", "", "", false},
		{"only commas clears all", ",,,", "", false},
		{"single tag too long (>16 runes)", "这个标签实在是太长太长超过十六个字了", "", true},
		{"total too long (>255 runes)", string(make255Plus()), "", true},
	}
	for _, c := range cases {
		got, err := sanitizeTags(c.in)
		if c.err {
			if err == nil {
				t.Errorf("%s: want error, got nil (out=%q)", c.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected err %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// make255Plus 构造总长超过 255 字符的合法标签串(每个标签 10 字符,26 个 -> 285 字符含逗号)。
func make255Plus() []rune {
	out := []rune{}
	for i := 0; i < 26; i++ {
		if i > 0 {
			out = append(out, ',')
		}
		for j := 0; j < 10; j++ {
			out = append(out, rune('a'+i))
		}
	}
	return out
}
