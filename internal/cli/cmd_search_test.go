package cli

import (
	"fmt"
	"testing"
)

func TestSearchSeeded(t *testing.T) {
	d := newVault(t)
	id := func(m map[string]any) string { return m["id"].(string) }
	poem := id(mustCreate(t, d, "source", `{"medium":"poem","title":"Trees","author":"A"}`))
	poemFr := id(mustCreate(t, d, "source", `{"medium":"poem","title":"Arbres","author":"A","translates":"`+poem+`"}`))
	paper := id(mustCreate(t, d, "source", `{"medium":"paper","title":"Paper","author":"B"}`))
	var first, reads, tr string
	for i := 0; i < 20; i++ {
		src, stance := paper, "asserts"
		switch {
		case i%3 == 0:
			src, stance = poem, "crafts"
		case i%3 == 1:
			src, stance = poem, "reads"
		}
		claim := fmt.Sprintf("generic remark number %d about things", i)
		ev := fmt.Sprintf("plain evidence text %d", i)
		if i == 7 {
			claim, ev = "the zebra pattern repeats", "zebra stripes zebra"
		}
		extra := ""
		if i == 4 {
			extra = fmt.Sprintf(`,"rereads":["%s"]`, first)
		}
		if i == 10 {
			src = poemFr
		}
		m := mustCreate(t, d, "note", fmt.Sprintf(`{"claim":%q,"evidence":%q,"stance":%q,"locator":"line %d","source":%q%s}`, claim, ev, stance, i+1, src, extra))
		switch i {
		case 0:
			first = id(m)
		case 4:
			reads = id(m)
		case 10:
			tr = id(m)
		}
	}
	search := func(args ...string) []map[string]any {
		t.Helper()
		rc, m, s := runCmd(d, append([]string{"search"}, args...)...)
		if rc != 0 {
			t.Fatalf("rc=%d %s", rc, s)
		}
		var out []map[string]any
		for _, h := range m["hits"].([]any) {
			out = append(out, h.(map[string]any))
		}
		return out
	}
	hits := search("zebra stripes")
	if len(hits) != 1 || hits[0]["claim"] != "the zebra pattern repeats" {
		t.Fatalf("%v", hits)
	}
	all := search("evidence text")
	if len(all) != 10 {
		t.Fatalf("default limit: %d", len(all))
	}
	for _, h := range search("evidence", "--medium", "poem", "--limit", "50") {
		if h["medium"] != "poem" {
			t.Fatal(h)
		}
	}
	rh := search("evidence", "--stance", "reads", "--limit", "50")
	if len(rh) == 0 {
		t.Fatal("no reads")
	}
	var sawReads, sawTr bool
	for _, h := range rh {
		if h["stance"] != "reads" {
			t.Fatal(h)
		}
		if h["id"] == reads {
			r, ok := h["rereads"].(map[string]any)
			if !ok || r["id"] != first || r["locator"] != "line 1" {
				t.Fatalf("%v", h)
			}
			sawReads = true
		}
	}
	for _, h := range search("evidence", "--limit", "50") {
		if h["id"] == tr {
			c, ok := h["counterpart"].(map[string]any)
			if !ok || c["id"] != poem || c["title"] != "Trees" {
				t.Fatalf("%v", h)
			}
			sawTr = true
		}
	}
	if !sawReads || !sawTr {
		t.Fatal("missing partner", sawReads, sawTr)
	}
	if n := len(search("evidence", "--limit", "50", "--budget", "30")); n == 0 || n >= len(search("evidence", "--limit", "50")) {
		t.Fatalf("budget: %d", n)
	}
	for _, args := range [][]string{{"x", "--stance", "shouts"}, {"x", "--medium", "zzz"}, {"x", "--hunch", "ghost"}} {
		if rc, _, _ := runCmd(d, append([]string{"search"}, args...)...); rc != 2 {
			t.Fatalf("%v rc=%d", args, rc)
		}
	}
}
