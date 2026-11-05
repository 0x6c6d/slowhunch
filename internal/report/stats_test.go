package report

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

func statsIndex(t *testing.T, notes map[string]string) *vault.Index {
	t.Helper()
	d := t.TempDir()
	for _, f := range []string{"notes", "sources", "hunches"} {
		os.MkdirAll(filepath.Join(d, f), 0o755)
	}
	w := func(p, s string) {
		if err := os.WriteFile(filepath.Join(d, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("sources/s1.md", "---\nid: s1\ntype: source\nmedium: poem\n---\n")
	w("sources/s2.md", "---\nid: s2\ntype: source\nmedium: paper\n---\n")
	for id, body := range notes {
		w("notes/"+id+".md", "---\nid: "+id+"\ntype: note\n"+body+"---\n")
	}
	w("hunches/h1.md", "---\nid: h1\ntype: hunch\nstate: open\n---\n")
	ix, _, err := (&vault.Vault{Root: d}).LoadIndex()
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func TestStatsCompute(t *testing.T) {
	ix := statsIndex(t, map[string]string{
		"a": "stance: asserts\nsource: s1\n",
		"b": "stance: asserts\nsource: s1\nstatus: contradiction\n",
		"c": "stance: reads\nsource: s2\n",
		"d": "stance: crafts\nsource: nope\n",
	})
	r1, r2 := New("r1"), New("r2")
	r1.Echoes = []Echo{{Atom: "a"}, {Atom: "b"}}
	r2.Echoes = []Echo{{Atom: "c"}}
	r1.AddCost("extract", Usage{TokensIn: 10, TokensOut: 5})
	r2.AddCost("extract", Usage{TokensIn: 1, TokensOut: 2})
	r2.AddCost("judge", Usage{TokensIn: 3, TokensOut: 4})
	s := Compute(ix, []*Report{r1, r2})
	if s.Atoms != 4 || s.Hunches != 1 || s.Contradictions != 1 || s.Echoes != 3 {
		t.Fatalf("%+v", s)
	}
	if s.AtomsByMedium["poem"] != 2 || s.AtomsByMedium["paper"] != 1 || s.AtomsByMedium["unknown"] != 1 {
		t.Fatalf("%v", s.AtomsByMedium)
	}
	if s.AtomsByStance["asserts"] != 2 || s.AtomsByStance["reads"] != 1 || s.AtomsByStance["crafts"] != 1 {
		t.Fatalf("%v", s.AtomsByStance)
	}
	if s.AssertsReadsRatio == nil || *s.AssertsReadsRatio != 2 {
		t.Fatalf("ratio %v", s.AssertsReadsRatio)
	}
	if s.TokensByRole["extract"] != (Usage{11, 7}) || s.TokensByRole["judge"] != (Usage{3, 4}) {
		t.Fatalf("%v", s.TokensByRole)
	}
	if s.HunchesByState["open"] != 1 {
		t.Fatal(s.HunchesByState)
	}
	ix2 := statsIndex(t, map[string]string{"a": "stance: asserts\nsource: s1\n"})
	s2 := Compute(ix2, nil)
	if s2.AssertsReadsRatio != nil || s2.TokensByRole == nil || s2.AtomsByMedium == nil {
		t.Fatalf("%+v", s2)
	}
}
