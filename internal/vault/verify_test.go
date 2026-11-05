package vault

import (
	"testing"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

func putText(t *testing.T, v *Vault, rel, text string) {
	t.Helper()
	d, err := frontmatter.Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.PutDoc(rel, d, ""); err != nil {
		t.Fatal(err)
	}
}

func verifyNote(id, extra string) string {
	return "---\nid: " + id + "\ntype: note\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nclaim: c\nstance: crafts\nsource: poem\n" + extra + "---\nbody\n"
}

func codesOf(ps []Problem) map[string]map[string]bool {
	m := map[string]map[string]bool{}
	for _, p := range ps {
		if m[p.ID] == nil {
			m[p.ID] = map[string]bool{}
		}
		m[p.ID][p.Code] = true
	}
	return m
}

func TestVerifyChecks(t *testing.T) {
	v := idxVault(t, nil)
	putText(t, v, "sources/poem.md", "---\nid: poem\ntype: source\nadded_by: human\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nmedium: poem\ntitle: P\ntext_hash: abc\n---\n")
	putText(t, v, "sources/nohash.md", "---\nid: nohash\ntype: source\nadded_by: human\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nmedium: poem\ntitle: P\n---\n")
	if err := v.WriteFile("inbox/poem.txt", FormatInbox("alpha\nbeta", "poem")); err != nil {
		t.Fatal(err)
	}
	putText(t, v, "notes/good.md", verifyNote("good", "evidence: alpha\nlocator: line 1\n"))
	putText(t, v, "notes/byhand.md", verifyNote("byhand", "evidence: zzz\nlocator: line 9\n")[:0]+
		"---\nid: byhand\ntype: note\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nclaim: c\nstance: crafts\nsource: nohash\nevidence: zzz\nlocator: line 9\n---\nbody\n")
	putText(t, v, "notes/badev.md", verifyNote("badev", "evidence: gamma\nlocator: line 1\n"))
	putText(t, v, "notes/range.md", verifyNote("range", "evidence: alpha\nlocator: lines 7-9\n"))
	putText(t, v, "notes/wrongline.md", verifyNote("wrongline", "evidence: alpha\nlocator: line 2\n"))
	putText(t, v, "notes/badloc.md", verifyNote("badloc", "evidence: alpha\nlocator: nonsense\n"))
	putText(t, v, "notes/dangle.md", verifyNote("dangle", "evidence: alpha\nlocator: line 1\n")[:len(verifyNote("dangle", "evidence: alpha\nlocator: line 1\n"))-5]+"see [[ghost]]\n")
	putText(t, v, "notes/badrev.md", verifyNote("badrev", "evidence: alpha\nlocator: line 1\n"))
	putText(t, v, "notes/badfield.md", verifyNote("badfield", "evidence: alpha\nlocator: line 1\n"))
	// corrupt by hand
	rewrite := func(rel, old, new string) {
		b, err := v.ReadFile(rel)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		i := indexOf(s, old)
		if i < 0 {
			t.Fatalf("%s: %q not found", rel, old)
		}
		if err := v.WriteFile(rel, []byte(s[:i]+new+s[i+len(old):])); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := v.ReadFile("notes/badrev.md")
	d, _ := frontmatter.Parse(b)
	rewrite("notes/badrev.md", "rev: "+d.GetString("rev"), "rev: 000000")
	rewrite("notes/badfield.md", "stance: crafts", "stance: bogus")
	// missing inbox on a second source
	putText(t, v, "sources/lost.md", "---\nid: lost\ntype: source\nadded_by: human\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nmedium: poem\ntitle: L\ntext_hash: def\n---\n")
	putText(t, v, "notes/lostnote.md", "---\nid: lostnote\ntype: note\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nclaim: c\nstance: crafts\nsource: lost\nevidence: x\nlocator: line 1\n---\nbody\n")
	v.WriteFile("notes/broken.md", []byte("no frontmatter"))

	ps, n, err := Verify(v)
	if err != nil {
		t.Fatal(err)
	}
	if n < 10 {
		t.Fatalf("checked=%d", n)
	}
	got := codesOf(ps)
	want := map[string]string{
		"broken": "BAD_FRONTMATTER", "badfield": "BAD_FIELD", "badrev": "REV_MISMATCH",
		"dangle": "DANGLING_LINK", "lost": "INBOX_MISSING", "badev": "EVIDENCE_MISMATCH",
		"range": "LOCATOR_RANGE", "wrongline": "LOCATOR_RANGE", "badloc": "LOCATOR_INVALID",
	}
	for id, code := range want {
		if !got[id][code] {
			t.Errorf("%s: want %s, got %v", id, code, got[id])
		}
	}
	for _, id := range []string{"good", "byhand", "poem", "nohash"} {
		if len(got[id]) != 0 {
			t.Errorf("%s should be clean: %v", id, got[id])
		}
	}
	for i := 1; i < len(ps); i++ {
		if ps[i-1].Path > ps[i].Path {
			t.Errorf("not sorted")
		}
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
