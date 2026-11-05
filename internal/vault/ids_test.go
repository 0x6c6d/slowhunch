package vault

import (
	"regexp"
	"strings"
	"testing"
)

func TestIDExamples(t *testing.T) {
	text := "The trees are coming into leaf"
	short := ShortHash(text)
	if len(short) != 4 {
		t.Fatalf("ShortHash len = %d", len(short))
	}
	cases := []struct{ got, want string }{
		{SourceID("poem", "Philip Larkin", "Trees", text), "poem-larkin-trees-" + short},
		{AtomID("crafts", "poem-larkin-trees-ab12", "lines 1-2", ""), "crafts-larkin-trees-lines-1-2"},
		{AtomID("reads", "own-notes-ab12", "", "A claim here"), "reads-notes-a-claim-here"},
		{AtomID("reads", "poem-larkin-trees-ab12", "timecode 1:23-1:31", ""), "reads-larkin-trees-timecode-1-23-1-31"},
		{AtomID("reads", "poem-larkin-trees-ab12", "", ""), "reads-larkin-trees-untitled"},
		{AtomID("reads", "", "lines 3", ""), "reads-lines-3"},
		{SourceID("own", "Me", "My Notes", text), "own-my-notes-" + short},
		{SourceID("quote", "", "Hi", text), "quote-anonymous-hi-" + short},
		{SourceID("quote", "X", "", text), "quote-x-untitled-" + short},
		{Slug("abcdefghijklmnopqrstuvwxyz0123", 24), "abcdefghijklmnopqrstuvwx"},
		{Slug("abcdefghijklmnopqrstuvw yz", 24), "abcdefghijklmnopqrstuvw"},
		{Slug("Ángström Ñandú Çava Straße Øre Æsir Über", 0), "angstrom-nandu-cava-strasse-ore-aesir-uber"},
		{KebabID("What makes a line stick?"), "what-makes-a-line-stick"},
		{KebabID("!!!"), "untitled"},
		{KebabID(strings.Repeat("word ", 30)), strings.TrimRight(strings.Repeat("word-", 12), "-")},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d: got %q, want %q", i, c.got, c.want)
		}
	}
	long := SourceID(strings.Repeat("m", 40), strings.Repeat("a", 40), strings.Repeat("t", 40), text)
	if long != strings.Repeat("m", 24)+"-"+strings.Repeat("a", 24)+"-"+strings.Repeat("t", 24)+"-"+short {
		t.Errorf("24-char cuts: %q", long)
	}
}

func TestUniqueID(t *testing.T) {
	taken := map[string]bool{"a": true, "a-2": true, "b": true}
	ex := func(s string) bool { return taken[s] }
	if got := UniqueID("c", ex); got != "c" {
		t.Errorf("got %q", got)
	}
	if got := UniqueID("b", ex); got != "b-2" {
		t.Errorf("got %q", got)
	}
	if got := UniqueID("a", ex); got != "a-3" {
		t.Errorf("got %q", got)
	}
}

func TestSlugSafety(t *testing.T) {
	ok := regexp.MustCompile(`^([a-z0-9]+(-[a-z0-9]+)*)?$`)
	inputs := []string{"../../etc/passwd", "/abs/path", "a\x00b", "C:\\x", "..", "\U0001F600\U0001F680",
		"%2e%2e/", "a/../../b", strings.Repeat("ab/", 170), ""}
	for _, s := range inputs {
		for _, max := range []int{24, 60} {
			got := Slug(s, max)
			if !ok.MatchString(got) || len(got) > max {
				t.Errorf("Slug(%q, %d) = %q", s, max, got)
			}
		}
		sid := SourceID("poem", s, s, s)
		for _, o := range []string{KebabID(s), sid, AtomID(s, sid, s, s), AtomID("reads", s, s, s)} {
			if !ok.MatchString(o) || o == "" {
				t.Errorf("input %q: unsafe id %q", s, o)
			}
		}
	}
}
