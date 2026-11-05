package intake

import "testing"

func TestNormalizeTable(t *testing.T) {
	cases := []struct{ name, in, medium, want string }{
		{"curly", "“it’s”", "article", "\"it's\""},
		{"dashes", "a—b–c…", "article", "a--b-c..."},
		{"nbsp", "a  b\ufeff\u200bc", "article", "a bc"},
		{"crlf", "a\r\nb\rc", "article", "a\nb\nc"},
		{"tabs", "\ta\t\tb  c \n", "article", "a b c"},
		{"blank runs", "\n\na\n\n\n\nb\n\n", "article", "a\n\nb"},
		{"poem", "line one\nline two", "poem", "line one\nline two"},
		{"code", "if x {\r\n\treturn  “y”\r\n}\n\n", "code", "if x {\n\treturn  “y”\n}"},
	}
	for _, c := range cases {
		if got := Normalize(c.in, c.medium); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestNormalizeIdempotent(t *testing.T) {
	inputs := []string{"a—b\n\n\n\nc  d", "  x\r\n\ty ", "“q”\t\n\n", "f() {\n\t\tx\n}\n\n"}
	for _, m := range []string{"article", "poem", "code", "chat"} {
		for _, in := range inputs {
			once := Normalize(in, m)
			if twice := Normalize(once, m); twice != once {
				t.Errorf("%s %q: %q != %q", m, in, twice, once)
			}
		}
	}
}

func TestTextHash(t *testing.T) {
	if got := TextHash("abc"); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("got %s", got)
	}
}
