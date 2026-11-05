package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func TestVerifyCorrupted(t *testing.T) {
	d := newVault(t)
	v, err := vault.Open(d)
	if err != nil {
		t.Fatal(err)
	}
	put := func(rel, text string) {
		doc, err := frontmatter.Parse([]byte(text))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.PutDoc(rel, doc, ""); err != nil {
			t.Fatal(err)
		}
	}
	put("sources/poem.md", "---\nid: poem\ntype: source\nadded_by: human\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nmedium: poem\ntitle: P\ntext_hash: abc\n---\n")
	if err := v.WriteFile("inbox/poem.txt", vault.FormatInbox("alpha\nbeta", "poem")); err != nil {
		t.Fatal(err)
	}
	note := func(id, ev, loc string) string {
		return "---\nid: " + id + "\ntype: note\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nclaim: c\nstance: crafts\nsource: poem\nevidence: " + ev + "\nlocator: " + loc + "\n---\nbody\n"
	}
	put("notes/n-one.md", note("n-one", "alpha", "line 1"))
	put("notes/n-two.md", note("n-two", "beta", "line 2"))
	rc, m, s := runCmd(d, "verify")
	if rc != 0 || len(m["problems"].([]any)) != 0 {
		t.Fatalf("clean: rc=%d %s", rc, s)
	}
	edit := func(rel, old, new string) {
		p := filepath.Join(d, rel)
		b, _ := os.ReadFile(p)
		if !strings.Contains(string(b), old) {
			t.Fatalf("%s lacks %q", rel, old)
		}
		os.WriteFile(p, []byte(strings.Replace(string(b), old, new, 1)), 0o644)
	}
	edit("notes/n-one.md", "evidence: alpha", "evidence: alphax")
	edit("notes/n-one.md", "body\n", "see [[ghost]]\n")
	edit("notes/n-two.md", "locator: line 2", "locator: lines 7-9")
	b, _ := os.ReadFile(filepath.Join(d, "notes/n-two.md"))
	doc, _ := frontmatter.Parse(b)
	edit("notes/n-two.md", "rev: "+doc.GetString("rev"), "rev: 000000")
	before := lintTreeHash(t, d)
	rc, m, s = runCmd(d, "verify")
	if rc != 3 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	found := map[string]bool{}
	for _, p := range m["problems"].([]any) {
		pm := p.(map[string]any)
		found[pm["id"].(string)+" "+pm["code"].(string)] = true
	}
	for _, k := range []string{"n-one EVIDENCE_MISMATCH", "n-one DANGLING_LINK", "n-two REV_MISMATCH", "n-two LOCATOR_RANGE"} {
		if !found[k] {
			t.Errorf("missing %s in %s", k, s)
		}
	}
	if lintTreeHash(t, d) != before {
		t.Fatal("verify modified the vault")
	}
}
