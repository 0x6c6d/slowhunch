package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// writeSource writes a valid source doc and its stored inbox text through the vault API.
func writeSource(t *testing.T, d, id, medium, text string) {
	t.Helper()
	v, err := vault.Open(d)
	if err != nil {
		t.Fatal(err)
	}
	doc := &frontmatter.Doc{}
	add := func(k string, val any) { doc.Fields = append(doc.Fields, frontmatter.Field{Key: k, Value: val}) }
	add("id", id)
	add("type", "source")
	add("created", "2026-11-01")
	add("updated", "2026-11-01")
	add("added_by", "human")
	add("rev", "000000")
	add("medium", medium)
	add("title", "Title of "+id)
	add("author", "Anon")
	if err := vault.Validate(doc, id); err != nil {
		t.Fatal(err)
	}
	if _, err := v.PutDoc("sources/"+id+".md", doc, ""); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteFile("inbox/"+id+".txt", vault.FormatInbox(text, medium)); err != nil {
		t.Fatal(err)
	}
}

func TestSourceFacts(t *testing.T) {
	d := newVault(t)
	const id = "poem-larkin-trees-ab12"
	writeSource(t, d, id, "poem", "The trees are coming into leaf\nLike something almost being said;")

	rc, m, s := runCmd(d, "source", "facts", id)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if m["id"] != id || m["medium"] != "poem" || m["count"] != float64(2) {
		t.Fatalf("bad result: %v", m)
	}
	lines, _ := m["lines"].([]any)
	if len(lines) != 2 {
		t.Fatalf("want 2 lines: %v", m)
	}
	for i, want := range []string{"The trees are coming into leaf", "Like something almost being said;"} {
		l := lines[i].(map[string]any)
		if l["n"] != float64(i+1) || l["text"] != want {
			t.Fatalf("line %d: %v", i, l)
		}
	}

	var out, errb bytes.Buffer
	if rc := Run([]string{"source", "facts", id, "--vault", d}, strings.NewReader(""), &out, &errb); rc != 0 {
		t.Fatalf("text rc=%d %s", rc, errb.String())
	}
	wantText := "   1  The trees are coming into leaf\n   2  Like something almost being said;\n"
	if out.String() != wantText {
		t.Fatalf("text %q", out.String())
	}

	// A plain medium is numbered on the fly.
	const art = "article-plain-cd34"
	writeSource(t, d, art, "article", "First plain line\nSecond plain line")
	rc, m, s = runCmd(d, "source", "facts", art)
	if rc != 0 {
		t.Fatalf("article rc=%d %s", rc, s)
	}
	lines, _ = m["lines"].([]any)
	if len(lines) != 2 || lines[0].(map[string]any)["n"] != float64(1) ||
		lines[1].(map[string]any)["n"] != float64(2) || lines[1].(map[string]any)["text"] != "Second plain line" {
		t.Fatalf("article lines: %v", m)
	}

	// Missing id is NOT_FOUND.
	rc, _, s = runCmd(d, "source", "facts", "no-such-source")
	if rc != 2 || !strings.Contains(s, "NOT_FOUND") {
		t.Fatalf("missing id rc=%d %s", rc, s)
	}

	// A non-source entry is BAD_INPUT.
	hid := mustCreate(t, d, "hunch", `{"question":"Why?"}`)["id"].(string)
	rc, _, s = runCmd(d, "source", "facts", hid)
	if rc != 2 || !strings.Contains(s, "BAD_INPUT") {
		t.Fatalf("non-source rc=%d %s", rc, s)
	}

	// A source with no stored text is NOT_FOUND with the hint.
	v, _ := vault.Open(d)
	if err := v.Remove("inbox/" + art + ".txt"); err != nil {
		t.Fatal(err)
	}
	var eb bytes.Buffer
	if rc := Run([]string{"source", "facts", art, "--vault", d}, strings.NewReader(""), &bytes.Buffer{}, &eb); rc != 2 ||
		!strings.Contains(eb.String(), "this source has no stored text") {
		t.Fatalf("no text rc=%d %s", rc, eb.String())
	}
}
