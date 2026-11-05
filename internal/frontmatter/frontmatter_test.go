package frontmatter

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

var docs = map[string]string{
	"note": `---
id: crafts-larkin-trees-lines-1-2
type: note
created: 2026-11-05
claim: "Short: quoted because of the colon"
quote: "He said \"hi\" # not a comment\nsecond line — ünïcødé ✓"
source: larkin-trees
tags: []
---
The trees are coming into leaf.
`,
	"source": `---
id: larkin-trees
type: source
title: The Trees (Philip Larkin)
url: "https://example.com/trees?x=1&y=2"
year: 1967
tags:
  - poetry
  - larkin
---
`,
	"hunch": `---
id: h1
type: hunch
claim: Grief is renewal disguised
confidence: 0.6
bears_on:
  - h2
  - "needs: quote"
evidence:
  - date: 2026-11-05
    note: crafts-larkin-trees-lines-1-2
    relation: supports
  - date: 2026-11-06
    note: other-note
    relation: contradicts
rev: a1b2c3
---
`,
	"proposal": `---
id: p-2026-11-05-1
type: proposal
status: open
target: h1
patch: "{\"claim\": \"new\"}"
reason: ""
evidence: []
---
`,
	"topic": `---
id: grief
type: topic
title: Grief
members:
  - h1
  - h2
---
# Grief

Some body text with: colons and "quotes".

- a list
`,
}

func TestRoundTrip(t *testing.T) {
	for name, src := range docs {
		d, err := Parse([]byte(src))
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		if got := string(d.Bytes()); got != src {
			t.Errorf("%s: round trip mismatch:\n%s\n---want---\n%s", name, got, src)
		}
		d2, err := Parse(d.Bytes())
		if err != nil || !reflect.DeepEqual(d, d2) {
			t.Errorf("%s: reparse differs: %v", name, err)
		}
	}
	d, _ := Parse([]byte(docs["note"]))
	if d.GetString("quote") != "He said \"hi\" # not a comment\nsecond line — ünïcødé ✓" {
		t.Errorf("quote = %q", d.GetString("quote"))
	}
	if l := d.GetList("tags"); l == nil || len(l) != 0 {
		t.Errorf("tags = %#v", l)
	}
	if m := d.GetMaps("tags"); m == nil || len(m) != 0 {
		t.Errorf("tags maps = %#v", m)
	}
	h, _ := Parse([]byte(docs["hunch"]))
	ev := h.GetMaps("evidence")
	if len(ev) != 2 || ev[1]["relation"] != "contradicts" {
		t.Errorf("evidence = %#v", ev)
	}
	if h.GetList("bears_on")[1] != "needs: quote" || h.GetList("evidence") != nil || h.GetString("bears_on") != "" {
		t.Errorf("accessors wrong")
	}
	// Writer sorts map keys and writes empty map lists as [].
	w := &Doc{Body: "body  \n\n\n"}
	w.Set("e", []map[string]string{{"z": "1", "a": "x y"}})
	w.Set("m", []map[string]string{})
	w.Set("s", "")
	want := "---\ne:\n  - a: x y\n    z: 1\nm: []\ns: \"\"\n---\nbody\n"
	if got := string(w.Bytes()); got != want {
		t.Errorf("writer:\n%q\nwant\n%q", got, want)
	}
	w.Delete("m")
	if _, ok := w.Get("m"); ok {
		t.Errorf("delete failed")
	}
}

func TestRevStable(t *testing.T) {
	d, err := Parse([]byte(docs["hunch"]))
	if err != nil {
		t.Fatal(err)
	}
	r := Rev(d)
	if !regexp.MustCompile(`^[0-9a-f]{6}$`).MatchString(r) {
		t.Fatalf("rev %q", r)
	}
	d2, _ := Parse(d.Bytes())
	if Rev(d2) != r {
		t.Errorf("rev changed after no-op round trip")
	}
	d2.Set("rev", "ffffff")
	if Rev(d2) != r {
		t.Errorf("rev depends on rev line")
	}
	d2.Delete("rev")
	if Rev(d2) != r {
		t.Errorf("rev depends on rev presence")
	}
	if d.GetString("rev") != "a1b2c3" {
		t.Errorf("Rev mutated doc")
	}
}

func TestRevChanges(t *testing.T) {
	d, _ := Parse([]byte(docs["hunch"]))
	r := Rev(d)
	c := d.Clone()
	c.Set("claim", "Something else")
	if Rev(c) == r {
		t.Errorf("rev unchanged after patch")
	}
	if Rev(d) != r {
		t.Errorf("clone not independent")
	}
	c = d.Clone()
	c.GetMaps("evidence")[0]["relation"] = "contradicts"
	if Rev(c) == r || Rev(d) != r {
		t.Errorf("deep clone / rev on map change wrong")
	}
}

func TestRejects(t *testing.T) {
	cases := map[string]string{
		"missing open":         "a: x\n---\n",
		"missing close":        "---\na: x\n",
		"tab":                  "---\n\ta: x\n---\n",
		"tab in value":         "---\na: \"x\"\t\n---\n",
		"block scalar |":       "---\na: |\n  x\n---\n",
		"block scalar >":       "---\na: >\n  x\n---\n",
		"flow map":             "---\na: {b: c}\n---\n",
		"flow list":            "---\na: [x, y]\n---\n",
		"anchor":               "---\na: &anchor x\n---\n",
		"alias":                "---\na: *anchor\n---\n",
		"tag":                  "---\na: !str x\n---\n",
		"comment line":         "---\n# c\na: x\n---\n",
		"inline comment":       "---\na: x # c\n---\n",
		"too deep":             "---\na:\n  - b:\n      - c\n---\n",
		"too deep map":         "---\na:\n  - b: x\n    c:\n      d: e\n---\n",
		"bad indent item":      "---\na:\n   - x\n---\n",
		"bad indent top":       "---\n a: x\n---\n",
		"bad indent cont":      "---\na:\n  - b: x\n     c: y\n---\n",
		"unterminated quote":   "---\na: \"abc\n---\n",
		"invalid quote":        "---\na: \"a\\q\"\n---\n",
		"trailing after quote": "---\na: \"a\" b\n---\n",
		"empty list key":       "---\na:\nb: x\n---\n",
		"empty list at end":    "---\na:\n---\n",
		"mixed list":           "---\na:\n  - x\n  - b: y\n---\n",
		"mixed list 2":         "---\na:\n  - b: y\n  - x\n---\n",
		"duplicate key":        "---\na: x\na: y\n---\n",
		"duplicate map key":    "---\na:\n  - b: x\n    b: y\n---\n",
		"CR":                   "---\r\na: x\r\n---\r\n",
		"bad key":              "---\nA: x\n---\n",
		"empty value":          "---\na: \n---\n",
		"no space":             "---\na:x\n---\n",
		"trailing space":       "---\na: x \n---\n",
	}
	for name, src := range cases {
		_, err := Parse([]byte(src))
		if err == nil {
			t.Errorf("%s: accepted %q", name, src)
			continue
		}
		if errs.CodeOf(err) != errs.BadInput {
			t.Errorf("%s: code %q", name, errs.CodeOf(err))
		}
		if !regexp.MustCompile(`line \d+`).MatchString(err.Error()) {
			t.Errorf("%s: error does not name line: %v", name, err)
		}
	}
}

func TestLineOf(t *testing.T) {
	b := []byte(docs["hunch"])
	if n := LineOf(b, "id"); n != 2 {
		t.Errorf("id line %d", n)
	}
	if n := LineOf(b, "evidence"); n != 9 {
		t.Errorf("evidence line %d", n)
	}
	if n := LineOf(b, "rev"); n != 16 {
		t.Errorf("rev line %d", n)
	}
	if n := LineOf(b, "date"); n != 0 {
		t.Errorf("nested key found at %d", n)
	}
	if n := LineOf(b, "missing"); n != 0 {
		t.Errorf("missing found at %d", n)
	}
	if n := LineOf([]byte(docs["topic"]), "members"); n != 5 {
		t.Errorf("members line %d", n)
	}
}
