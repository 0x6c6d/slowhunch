package synth

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

func cc(n int) vault.CollideConfig {
	return vault.CollideConfig{MaxCosine: 0.15, AestheticWeight: 2.0, MaxPairs: n}
}

func TestCollideSelect(t *testing.T) {
	// threshold and aesthetic weighting
	items := []Item{
		{ID: "a", Medium: "paper", Text: "alpha beta gamma"},
		{ID: "b", Medium: "paper", Text: "delta epsilon zeta"},
		{ID: "c", Medium: "poem", Aesthetic: true, Text: "eta theta iota"},
		{ID: "d", Medium: "paper", Text: "alpha beta gamma"},
	}
	sel := Select(items, nil, cc(6), 0)
	for _, p := range sel.Pairs {
		if p.Cosine > 0.15 {
			t.Fatalf("cosine too high: %+v", p)
		}
		if p.A >= p.B {
			t.Fatalf("unordered pair %+v", p)
		}
	}
	if sel.Candidates != 5 { // a-d is identical, excluded
		t.Fatalf("candidates %d", sel.Candidates)
	}
	top := sel.Pairs[0]
	if !(top.A == "c" || top.B == "c") {
		t.Fatalf("aesthetic pair should rank first: %+v", sel.Pairs)
	}

	// max_pairs and capped
	var many []Item
	for _, id := range []string{"p", "q", "r", "s", "t"} {
		many = append(many, Item{ID: id, Medium: "paper", Text: "w" + id + " x" + id})
	}
	sel = Select(many, nil, cc(6), 0)
	if len(sel.Pairs) != 6 || !sel.Capped || sel.Candidates != 10 {
		t.Fatalf("cap: %+v", sel)
	}
	if got := Select(many, nil, cc(6), 2); len(got.Pairs) != 2 {
		t.Fatalf("limit 2: %d", len(got.Pairs))
	}
	if got := Select(many, nil, cc(6), 99); len(got.Pairs) != 6 {
		t.Fatalf("limit 99: %d", len(got.Pairs))
	}

	// existing pairs skipped in both orders
	ex := map[[2]string]bool{{"p", "q"}: true, {"s", "r"}: true}
	got := Select(many, ex, cc(20), 0)
	if got.Candidates != 8 {
		t.Fatalf("existing: %d", got.Candidates)
	}

	// reserved cross-medium asserts x reads despite lower score
	res := []Item{
		{ID: "a1", Type: "note", Medium: "poem", Aesthetic: true, Stance: "crafts", Text: "w1"},
		{ID: "a2", Type: "note", Medium: "poem", Aesthetic: true, Stance: "crafts", Text: "w2"},
		{ID: "a3", Type: "note", Medium: "poem", Aesthetic: true, Stance: "crafts", Text: "w3"},
		{ID: "a4", Type: "note", Medium: "poem", Aesthetic: true, Stance: "crafts", Text: "w4"},
		{ID: "x1", Type: "note", Medium: "paper", Stance: "asserts", Text: "w5"},
		{ID: "y1", Type: "note", Medium: "blog", Stance: "reads", Text: "w6"},
	}
	sel = Select(res, nil, cc(4), 0)
	found := false
	for _, p := range sel.Pairs {
		if p.A == "x1" && p.B == "y1" {
			found = p.Reserved
		}
	}
	if !found || len(sel.Pairs) != 4 {
		t.Fatalf("reserved missing: %+v", sel)
	}
	// same-medium asserts x reads is not reserved
	res[5].Medium = "paper"
	sel = Select(res, nil, cc(4), 0)
	for _, p := range sel.Pairs {
		if p.Reserved || (p.A == "x1" && p.B == "y1") {
			t.Fatalf("same medium reserved: %+v", sel)
		}
	}

	// determinism
	if !reflect.DeepEqual(Select(many, ex, cc(6), 0), Select(many, ex, cc(6), 0)) {
		t.Fatal("not deterministic")
	}

	// Pool and ExistingPairs from a vault
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"sources/s-poem.md":    "---\nid: s-poem\ntype: source\nmedium: poem\n---\n",
		"notes/n-1.md":         "---\nid: n-1\ntype: note\nclaim: the claim\nstance: asserts\nsource: s-poem\n---\nbody one\n",
		"hunches/h-open.md":    "---\nid: h-open\ntype: hunch\nquestion: why\nstate: open\ncreated: 2026-01-01\n---\n",
		"hunches/h-tested.md":  "---\nid: h-tested\ntype: hunch\nquestion: why\nstate: tested\ncreated: 2026-01-01\n---\n",
		"proposals/p-1.md":     "---\nid: p-1\ntype: proposal\nitems:\n  - n-1\n  - h-open\nidea: i\nstatus: open\n---\n",
		"proposals/p-short.md": "---\nid: p-short\ntype: proposal\nitems:\n  - n-1\nidea: i\nstatus: open\n---\n",
	}
	for p, c := range files {
		if err := v.WriteFile(p, []byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	ix, lerrs, err := v.LoadIndex()
	if err != nil || len(lerrs) != 0 {
		t.Fatalf("index: %v %v", err, lerrs)
	}
	pool := Pool(ix)
	if len(pool) != 2 || pool[0].ID != "h-open" || pool[1].ID != "n-1" {
		t.Fatalf("pool: %+v", pool)
	}
	if n := pool[1]; n.Stance != "asserts" || n.Medium != "poem" || !n.Aesthetic || n.Text != "the claim\nbody one" {
		t.Fatalf("note item: %+v", n)
	}
	e := ExistingPairs(ix)
	if len(e) != 2 || !e[[2]string{"n-1", "h-open"}] || !e[[2]string{"h-open", "n-1"}] {
		t.Fatalf("existing: %v", e)
	}
}
