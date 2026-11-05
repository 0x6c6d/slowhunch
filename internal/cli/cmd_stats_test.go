package cli

import (
	"testing"

	"github.com/0x6c6d/slowhunch/internal/report"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func TestStatsCommand(t *testing.T) {
	d := newVault(t)
	id := func(m map[string]any) string { return m["id"].(string) }
	art := id(mustCreate(t, d, "source", `{"medium":"article","title":"Art","author":"A"}`))
	poem := id(mustCreate(t, d, "source", `{"medium":"poem","title":"Poem","author":"B"}`))
	note := func(c, st, src string) string {
		loc := "line 1"
		if src == art {
			loc = "section intro"
		}
		return id(mustCreate(t, d, "note", `{"claim":"`+c+`","evidence":"e","stance":"`+st+`","locator":"`+loc+`","source":"`+src+`"}`))
	}
	note("a one", "asserts", art)
	note("a two", "asserts", art)
	mustCreate(t, d, "note", `{"claim":"a three","evidence":"e","stance":"asserts","locator":"section intro","source":"`+art+`","status":"contradiction"}`)
	note("c one", "crafts", poem)
	note("c two", "crafts", poem)
	note("r one", "reads", poem)
	addHunch(t, d, "First hunch?")
	mustCreate(t, d, "hunch", `{"question":"Second hunch?","state":"warming"}`)
	v, err := vault.Open(d)
	if err != nil {
		t.Fatal(err)
	}
	r := report.New("run-1")
	r.Echoes = []report.Echo{{Atom: "x", Echoes: "y", Cosine: 0.9}}
	r.AddCost("extract", report.Usage{TokensIn: 7, TokensOut: 3})
	if err := r.Save(v); err != nil {
		t.Fatal(err)
	}
	rc, m, s := runCmd(d, "stats")
	if rc != 0 {
		t.Fatal(s)
	}
	num := func(x any) int { return int(x.(float64)) }
	med := m["atoms_by_medium"].(map[string]any)
	st := m["atoms_by_stance"].(map[string]any)
	hs := m["hunches_by_state"].(map[string]any)
	tok := m["tokens_by_role"].(map[string]any)["extract"].(map[string]any)
	if num(med["article"]) != 3 || num(med["poem"]) != 3 || num(st["asserts"]) != 3 || num(st["crafts"]) != 2 || num(st["reads"]) != 1 {
		t.Fatalf("%s", s)
	}
	if m["asserts_reads_ratio"].(float64) != 3 || num(m["contradictions"]) != 1 || num(m["echoes"]) != 1 ||
		num(m["atoms"]) != 6 || num(m["hunches"]) != 2 || num(hs["open"]) != 1 || num(hs["warming"]) != 1 ||
		num(tok["tokens_in"]) != 7 || num(tok["tokens_out"]) != 3 {
		t.Fatalf("%s", s)
	}
}
