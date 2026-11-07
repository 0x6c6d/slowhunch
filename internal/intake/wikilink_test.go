package intake

import "testing"

func TestWikilinkStrip(t *testing.T) {
	cases := []struct{ in, want string }{
		{"[[a]]", "a"},
		{"[[a|b]]", "a|b"},
		{"![[a]]", "a"},
		{"[[a#h]]", "a#h"},
		{"see [[a]] and [[b]]", "see a and b"},
		{"[[Blood Hound]]", "Blood Hound"},
		{"[[[[a]]]]", "a"},
		{"[x]", "[x]"},
		{"[t](u)", "[t](u)"},
		{"[[", "[["},
		{"[[]]", "[[]]"},
		{"[[a\nb]]", "[[a\nb]]"},
	}
	for _, c := range cases {
		got := Normalize(c.in, "article")
		if got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
		if again := Normalize(got, "article"); again != got {
			t.Errorf("not idempotent for %q: %q -> %q", c.in, got, again)
		}
	}
}

func TestWikilinkCodeUntouched(t *testing.T) {
	if got := Normalize("m[[i]] = 1", "code"); got != "m[[i]] = 1" {
		t.Errorf("code changed: %q", got)
	}
	if got := Normalize("m[[i]] = 1", "prose"); got != "mi = 1" {
		t.Errorf("prose: %q", got)
	}
}

func TestWikilinkHash(t *testing.T) {
	h := TextHash(Normalize("see [[a]]", "article"))
	if h == TextHash("see [[a]]") {
		t.Error("hash unchanged by wikilink strip")
	}
	if h != TextHash("see a") {
		t.Error("hash differs from stripped text hash")
	}
}
