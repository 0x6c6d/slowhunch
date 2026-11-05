package vault

import "testing"

func lintNote(id, created, extra string) string {
	return "---\nid: " + id + "\ntype: note\ncreated: " + created + "\nclaim: c\n" + extra + "---\nbody\n"
}

func lintCodes(t *testing.T, files map[string]string) map[string][]string {
	t.Helper()
	ix, le := load(t, idxVault(t, files))
	got := map[string][]string{}
	for _, p := range Lint(ix, le) {
		got[p.ID] = append(got[p.ID], p.Code)
	}
	return got
}

func TestLintRules(t *testing.T) {
	src := func(id, medium string) string {
		return "---\nid: " + id + "\ntype: source\nmedium: " + medium + "\ntitle: t\n---\n"
	}
	files := map[string]string{
		"sources/poem.md":  src("poem", "poem"),
		"sources/paper.md": src("paper", "paper"),
		// positives
		"notes/a-noloc.md":  lintNote("a-noloc", "2026-01-01", "stance: crafts\nsource: poem\nevidence: e1\nlocator: \"\"\n"),
		"notes/b-conf.md":   lintNote("b-conf", "2026-01-01", "stance: crafts\nsource: paper\nevidence: e2\nlocator: l\nconfidence: \"0.5\"\n"),
		"notes/c-rend.md":   lintNote("c-rend", "2026-01-01", "stance: asserts\nsource: paper\nevidence: e3\nlocator: l\nrendering: e3\n"),
		"notes/d-rend0.md":  lintNote("d-rend0", "2026-01-01", "stance: asserts\nsource: paper\nevidence: \"\"\nlocator: l\nrendering: r\n"),
		"notes/e-first.md":  lintNote("e-first", "2026-01-01", "stance: asserts\nsource: paper\nevidence: same\nlocator: l\n"),
		"notes/f-reads.md":  lintNote("f-reads", "2026-01-02", "stance: reads\nsource: paper\nevidence: same\nlocator: l\n"),
		"notes/g-reads2.md": lintNote("g-reads2", "2026-01-02", "stance: reads\nsource: paper\nevidence: same\nlocator: l\nrereads:\n  - e-first\n  - f-reads\n"),
		// negatives
		"notes/h-ok.md":    lintNote("h-ok", "2026-01-01", "stance: asserts\nsource: poem\nevidence: ok\nlocator: line 1\nconfidence: \"0.5\"\nrendering: other\n"),
		"notes/i-paper.md": lintNote("i-paper", "2026-01-01", "stance: asserts\nsource: paper\nevidence: ok2\nlocator: \"\"\n"),
		"notes/bad.md":     "no frontmatter",
	}
	got := lintCodes(t, files)
	want := map[string][]string{
		"a-noloc": {"MISSING_LOCATOR"},
		"b-conf":  {"CONFIDENCE_NOT_ASSERTS"},
		"c-rend":  {"RENDERING_AS_EVIDENCE"},
		"d-rend0": {"RENDERING_AS_EVIDENCE"},
		"f-reads": {"READS_WITHOUT_REREADS"},
		"bad":     {"BAD_FRONTMATTER"},
	}
	for id, w := range want {
		if len(got[id]) != 1 || got[id][0] != w[0] {
			t.Errorf("%s: got %v want %v", id, got[id], w)
		}
	}
	for _, id := range []string{"e-first", "g-reads2", "h-ok", "i-paper"} {
		if len(got[id]) != 0 {
			t.Errorf("%s should be clean: %v", id, got[id])
		}
	}
}
