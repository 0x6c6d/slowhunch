package vault

import (
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func idxVault(t *testing.T, files map[string]string) *Vault {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for p, c := range files {
		if err := v.WriteFile(p, []byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	return v
}

func load(t *testing.T, v *Vault) (*Index, []LoadError) {
	t.Helper()
	ix, le, err := v.LoadIndex()
	if err != nil {
		t.Fatal(err)
	}
	return ix, le
}

func TestIndexLoad(t *testing.T) {
	v := idxVault(t, map[string]string{
		"notes/n1.md":       "---\nid: n1\ntype: note\n---\nbody\n",
		"sources/s1.md":     "---\nid: s1\ntype: source\nmedium: paper\n---\n",
		"hunches/h1.md":     "---\nid: h1\ntype: hunch\nquestion: q\n---\n",
		"proposals/p1.md":   "---\nid: p1\ntype: proposal\n---\n",
		"topics/t1.md":      "---\nid: t1\ntype: topic\ntitle: T\n---\n",
		"archive/old.md":    "---\nid: old\ntype: note\n---\n",
		"topics/bad.md":     "---\nid: bad\ntype: topic\ntitle: |\n  x\n---\n",
		"notes/wrong.md":    "---\nid: wrong\ntype: hunch\nquestion: q\n---\n",
		"notes/mismatch.md": "---\nid: other\ntype: note\n---\n",
		"topics/n1.md":      "---\nid: n1\ntype: topic\ntitle: T\n---\n",
	})
	ix, le := load(t, v)
	var ids []string
	for _, e := range ix.All() {
		ids = append(ids, e.ID)
	}
	if want := []string{"h1", "n1", "p1", "s1", "t1"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids %v want %v", ids, want)
	}
	if len(le) != 4 {
		t.Fatalf("load errors %v", le)
	}
	if _, ok := ix.Get("old"); ok {
		t.Fatal("archive indexed")
	}
	if e, _ := ix.Get("n1"); e.Path != "notes/n1.md" || e.Type != "note" {
		t.Fatalf("entry %+v", e)
	}
	if len(ix.OfType("topic")) != 1 {
		t.Fatal("OfType")
	}
}

func TestBacklinks(t *testing.T) {
	v := idxVault(t, map[string]string{
		"notes/a.md":   "---\nid: a\ntype: note\n---\nsee [[t]]\n",
		"notes/b.md":   "---\nid: b\ntype: note\nsource: s\n---\n",
		"notes/c.md":   "---\nid: c\ntype: note\nlinks:\n  - t\n---\n",
		"sources/s.md": "---\nid: s\ntype: source\nmedium: blog\n---\n",
		"topics/t.md":  "---\nid: t\ntype: topic\ntitle: T\n---\n",
		"hunches/h.md": hunchDoc(),
	})
	ix, _ := load(t, v)
	if got := ix.Backlinks("t"); !reflect.DeepEqual(got, []string{"a", "c", "h"}) {
		t.Fatalf("t: %v", got)
	}
	if got := ix.Backlinks("s"); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("s: %v", got)
	}
	if got := ix.Backlinks("a"); !reflect.DeepEqual(got, []string{"h"}) {
		t.Fatalf("a: %v", got)
	}
	if got := ix.Outgoing("b"); !reflect.DeepEqual(got, []string{"s"}) {
		t.Fatalf("out: %v", got)
	}
}

func TestNeighbourhood(t *testing.T) {
	v := idxVault(t, map[string]string{
		"notes/a.md": "---\nid: a\ntype: note\n---\n[[b]]\n",
		"notes/b.md": "---\nid: b\ntype: note\n---\n[[c]]\n",
		"notes/c.md": "---\nid: c\ntype: note\n---\n[[d]]\n",
		"notes/d.md": "---\nid: d\ntype: note\n---\n",
	})
	ix, _ := load(t, v)
	n, e := ix.Neighbourhood("b", 1)
	if !reflect.DeepEqual(n, []string{"a", "b", "c"}) || !reflect.DeepEqual(e, [][2]string{{"a", "b"}, {"b", "c"}}) {
		t.Fatalf("d1 %v %v", n, e)
	}
	n, e = ix.Neighbourhood("b", 2)
	if !reflect.DeepEqual(n, []string{"a", "b", "c", "d"}) || len(e) != 3 {
		t.Fatalf("d2 %v %v", n, e)
	}
}

func TestMediumOf(t *testing.T) {
	v := idxVault(t, map[string]string{
		"sources/s.md": "---\nid: s\ntype: source\nmedium: poem\n---\n",
		"notes/n.md":   "---\nid: n\ntype: note\nsource: s\n---\n",
		"notes/m.md":   "---\nid: m\ntype: note\n---\n",
	})
	ix, _ := load(t, v)
	if ix.MediumOf("s") != "poem" || ix.MediumOf("n") != "poem" || ix.MediumOf("m") != "" || ix.MediumOf("zz") != "" {
		t.Fatal("MediumOf")
	}
}

func hunchDoc() string {
	d := &frontmatter.Doc{}
	d.Set("id", "h")
	d.Set("type", "hunch")
	d.Set("question", "q")
	d.Set("bears_on", []string{"t"})
	d.Set("evidence", []map[string]string{{"date": "2026-11-05", "note": "see [[a]]", "relation": "supports"}})
	return string(d.Bytes())
}
