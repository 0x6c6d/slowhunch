package vault

import (
	"reflect"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

func mk(typ string, kv ...string) *frontmatter.Doc {
	d := &frontmatter.Doc{}
	base := []string{"id", "x-1", "type", typ, "created", "2026-11-05", "updated", "2026-11-05", "rev", "abc123"}
	for i := 0; i < len(base); i += 2 {
		d.Set(base[i], base[i+1])
	}
	if typ == "source" {
		d.Set("added_by", "agent:claude/1")
	} else {
		d.Set("author", "human")
	}
	for i := 0; i < len(kv); i += 2 {
		d.Set(kv[i], kv[i+1])
	}
	return d
}

func validDocs() map[string]*frontmatter.Doc {
	h := mk("hunch", "question", "q?", "state", "open")
	h.Set("evidence", []map[string]string{})
	return map[string]*frontmatter.Doc{
		"note":     mk("note", "claim", "c", "evidence", "e", "stance", "asserts", "confidence", "0.5", "locator", "line 2"),
		"hunch":    h,
		"source":   mk("source", "medium", "poem", "title", "T", "author", "Someone"),
		"proposal": mk("proposal", "idea", "i", "status", "open"),
		"topic":    mk("topic", "title", "T"),
	}
}

func TestValidateTypes(t *testing.T) {
	for typ, d := range validDocs() {
		if err := Validate(d, "x-1"); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
	}
	n := func() *frontmatter.Doc { return validDocs()["note"] }
	cases := map[string]func() (*frontmatter.Doc, string){
		"unknown field":    func() (*frontmatter.Doc, string) { d := n(); d.Set("bogus", "x"); return d, "x-1" },
		"missing required": func() (*frontmatter.Doc, string) { d := n(); d.Delete("claim"); return d, "x-1" },
		"bad enum":         func() (*frontmatter.Doc, string) { d := n(); d.Set("stance", "maybe"); return d, "x-1" },
		"bad date":         func() (*frontmatter.Doc, string) { d := n(); d.Set("created", "2026-02-30"); return d, "x-1" },
		"confidence crafts": func() (*frontmatter.Doc, string) {
			d := n()
			d.Set("stance", "crafts")
			return d, "x-1"
		},
		"id mismatch": func() (*frontmatter.Doc, string) { return n(), "other" },
		"bad locator": func() (*frontmatter.Doc, string) { d := n(); d.Set("locator", "page 4"); return d, "x-1" },
		"bad author":  func() (*frontmatter.Doc, string) { d := n(); d.Set("author", "bob"); return d, "x-1" },
		"multi claim": func() (*frontmatter.Doc, string) { d := n(); d.Set("claim", "a\nb"); return d, "x-1" },
		"bad id list": func() (*frontmatter.Doc, string) { d := n(); d.Set("links", []string{"Bad Id"}); return d, "x-1" },
		"list scalar": func() (*frontmatter.Doc, string) { d := n(); d.Set("tags", "x"); return d, "x-1" },
	}
	for name, f := range cases {
		d, id := f()
		err := Validate(d, id)
		if err == nil || errs.CodeOf(err) != errs.BadInput {
			t.Errorf("%s: want BAD_INPUT, got %v", name, err)
		}
	}
	if err := Validate(n(), "x-1"); err != nil {
		t.Fatal(err)
	}
	d := n()
	d.Set("bogus", "x")
	if err := Validate(d, "x-1"); err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Errorf("unknown field not named: %v", err)
	}
	h := validDocs()["hunch"]
	h.Set("evidence", []map[string]string{{"date": "2026-11-05", "note": "n", "relation": "nope"}})
	if Validate(h, "x-1") == nil {
		t.Error("bad relation accepted")
	}
}

func TestLocatorGrammar(t *testing.T) {
	for _, s := range []string{"line 3", "lines 1-2", "lines 2-2", "stanza 2", "verse 1", "comment 4", "section Intro to it", "timecode 1:23-1:31", "region 1,2,30,40"} {
		if _, err := ParseLocator(s); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
	for _, s := range []string{"", "line", "lines 3-1", "line 0", "page 4", "timecode 1:61-2:00", "timecode 2:00-1:00", "region 5,5,1,1", "region 1,1,1,2", "section ", "section a\nb", "section " + strings.Repeat("x", 81), "line -1", "line 1 2"} {
		if _, err := ParseLocator(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
	l, _ := ParseLocator("lines 3-5")
	if f, to, ok := l.LineRange(); !ok || f != 3 || to != 5 {
		t.Error("range")
	}
	l, _ = ParseLocator("stanza 2")
	if _, _, ok := l.LineRange(); ok {
		t.Error("stanza range")
	}
}

func TestLinks(t *testing.T) {
	d := &frontmatter.Doc{Body: "a [[x-1]] b [[y-2]] [[x-1]] [[Bad]]\n"}
	d.Set("source", "s-1")
	d.Set("bears_on", []string{"y-2", "h-1"})
	d.Set("evidence", []map[string]string{{"note": "see [[n-9]]"}})
	got := Links(d)
	want := []string{"x-1", "y-2", "s-1", "h-1", "n-9"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestNoteBody(t *testing.T) {
	ev := "one\n\nthree"
	b := NoteBody("claim", ev)
	if b != "claim\n\n> one\n>\n> three\n" {
		t.Fatalf("%q", b)
	}
	lines := strings.Split(strings.TrimSuffix(strings.SplitN(b, "\n\n", 2)[1], "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimPrefix(strings.TrimPrefix(l, ">"), " ")
	}
	if strings.Join(lines, "\n") != ev {
		t.Error("round trip")
	}
}

func TestInboxFormat(t *testing.T) {
	st := FormatInbox("a\n\nb", "poem")
	if string(st) != "1\ta\n2\t\n3\tb\n" {
		t.Fatalf("%q", st)
	}
	if got := InboxPlain(st, "poem"); got != "a\n\nb" {
		t.Errorf("%q", got)
	}
	pl := FormatInbox("x\ny", "article")
	if string(pl) != "x\ny\n" || InboxPlain(pl, "article") != "x\ny" {
		t.Errorf("%q", pl)
	}
	if len(InboxLines(pl, "article")) != 2 || !NumberedMedium("code") || NumberedMedium("paper") {
		t.Error("lines")
	}
	if Folder("topic") != "topics" || Folder("zz") != "" || !IsAesthetic("art") || !IsFactual("book") || IsFactual("own") || IsAesthetic("other") {
		t.Error("vocab")
	}
}

func TestValidateCorroborations(t *testing.T) {
	ok := validDocs()["note"]
	ok.Set("corroborations", []string{"2026-11-05 poem-larkin-trees-ab12"})
	if err := Validate(ok, "x-1"); err != nil {
		t.Fatalf("valid entry rejected: %v", err)
	}
	for _, e := range []string{"not-a-date x", "2026-11-05 Bad_ID", "2026-13-45 x", "poem-larkin-trees-ab12"} {
		d := validDocs()["note"]
		d.Set("corroborations", []string{e})
		err := Validate(d, "x-1")
		if err == nil || !strings.Contains(err.Error(), "corroborations") {
			t.Fatalf("entry %q: want error naming field, got %v", e, err)
		}
	}
}
