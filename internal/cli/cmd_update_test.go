package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readVaultFile(t *testing.T, d, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(d, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func commitCount(t *testing.T, d string) int {
	t.Helper()
	return len(strings.Fields(gitOut(t, d, "log", "--format=%H")))
}

func TestCRUDUpdatePatch(t *testing.T) {
	d := newVault(t)
	m := mustCreate(t, d, "topic", `{"title":"Trees"}`)
	before := commitCount(t, d)
	rc, r, s := runCmd(d, "update", "trees", "--rev", m["rev"].(string), "--data", `{"tags":["poetry","nature"]}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if r["rev"] == m["rev"] || r["rev"] == "" || r["path"] != "topics/trees.md" {
		t.Fatalf("bad result %v", r)
	}
	body := readVaultFile(t, d, "topics/trees.md")
	if !strings.Contains(body, "  - poetry") || !strings.Contains(body, "updated: "+time.Now().Format("2006-01-02")) {
		t.Fatalf("file: %s", body)
	}
	if !strings.Contains(body, "rev: "+r["rev"].(string)) {
		t.Fatalf("rev not stored: %s", body)
	}
	if n := commitCount(t, d); n != before+1 {
		t.Fatalf("want one new commit, got %d", n-before)
	}
	if !strings.HasPrefix(gitOut(t, d, "log", "-1", "--format=%s"), "slowhunch: update trees") {
		t.Fatal(gitOut(t, d, "log", "-1", "--format=%s"))
	}
	// same patch again with the new rev changes nothing: no write, no commit
	rc, r2, s := runCmd(d, "update", "trees", "--rev", r["rev"].(string), "--data", `{"tags":["poetry","nature"]}`)
	if rc != 0 || r2["rev"] != r["rev"] || commitCount(t, d) != before+1 {
		t.Fatalf("no-op: rc=%d %s", rc, s)
	}
}

func TestCRUDUpdateStaleRev(t *testing.T) {
	d := newVault(t)
	m := mustCreate(t, d, "topic", `{"title":"Trees"}`)
	old := m["rev"].(string)
	if rc, _, s := runCmd(d, "update", "trees", "--rev", old, "--data", `{"tags":["a"]}`); rc != 0 {
		t.Fatal(s)
	}
	cur := readVaultFile(t, d, "topics/trees.md")
	n := commitCount(t, d)
	rc, r, s := runCmd(d, "update", "trees", "--rev", old, "--data", `{"tags":["b"]}`)
	if rc != 4 || !strings.Contains(s, "CONFLICT_REV") {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if !strings.Contains(s, `"current"`) {
		t.Fatalf("no current: %s", s)
	}
	var found bool
	var walk func(any)
	walk = func(x any) {
		switch y := x.(type) {
		case map[string]any:
			if c, ok := y["current"].(string); ok && c == cur {
				found = true
			}
			for _, z := range y {
				walk(z)
			}
		}
	}
	walk(r)
	if !found {
		t.Fatalf("current content not in error: %s", s)
	}
	if readVaultFile(t, d, "topics/trees.md") != cur || commitCount(t, d) != n {
		t.Fatal("file or history changed")
	}
}

func TestCRUDUpdateNoRev(t *testing.T) {
	d := newVault(t)
	mustCreate(t, d, "topic", `{"title":"Trees"}`)
	before := readVaultFile(t, d, "topics/trees.md")
	rc, _, s := runCmd(d, "update", "trees", "--data", `{"tags":["x"]}`)
	if rc != 2 || !strings.Contains(s, "--rev") {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if readVaultFile(t, d, "topics/trees.md") != before {
		t.Fatal("file changed")
	}
	m := mustCreate(t, d, "topic", `{"title":"Other"}`)
	if rc, _, _ := runCmd(d, "update", "other", "--rev", m["rev"].(string)); rc != 2 {
		t.Fatal("neither --data nor --append accepted")
	}
	if rc, _, _ := runCmd(d, "update", "other", "--rev", m["rev"].(string), "--data", `{}`, "--append", "x"); rc != 2 {
		t.Fatal("both --data and --append accepted")
	}
	if rc, _, _ := runCmd(d, "update", "other", "--rev", m["rev"].(string), "--data", `{"id":"x"}`); rc != 2 {
		t.Fatal("id patch accepted")
	}
	if rc, _, s := runCmd(d, "update", "nope", "--rev", "000000", "--data", `{}`); rc != 2 || !strings.Contains(s, "NOT_FOUND") {
		t.Fatalf("missing id: %d %s", rc, s)
	}
}

func TestCRUDUpdateEvidence(t *testing.T) {
	d := newVault(t)
	m := mustCreate(t, d, "note", `{"claim":"c","evidence":"original words","stance":"asserts"}`)
	id := m["id"].(string)
	rel := "notes/" + id + ".md"
	before := readVaultFile(t, d, rel)
	for _, p := range []string{`{"evidence":"rewritten"}`, `{"evidence":null}`} {
		rc, _, s := runCmd(d, "update", id, "--rev", m["rev"].(string), "--data", p)
		if rc != 2 || !strings.Contains(s, "evidence is never rewritten") {
			t.Fatalf("%s rc=%d %s", p, rc, s)
		}
	}
	if readVaultFile(t, d, rel) != before {
		t.Fatal("file changed")
	}
	// claim change regenerates the body
	rc, _, s := runCmd(d, "update", id, "--rev", m["rev"].(string), "--data", `{"claim":"new claim"}`)
	if rc != 0 {
		t.Fatal(s)
	}
	if !strings.HasSuffix(readVaultFile(t, d, rel), "new claim\n\n> original words\n") {
		t.Fatal(readVaultFile(t, d, rel))
	}
}

func TestCRUDUpdateAppend(t *testing.T) {
	d := newVault(t)
	m := mustCreate(t, d, "topic", `{"title":"Trees","body":"First line."}`)
	rc, r, s := runCmd(d, "update", "trees", "--rev", m["rev"].(string), "--append", "  Second line.  ")
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	got := readVaultFile(t, d, "topics/trees.md")
	if !strings.HasSuffix(got, "First line.\n\nSecond line.\n") {
		t.Fatalf("%q", got)
	}
	if rc, _, _ := runCmd(d, "update", "trees", "--rev", r["rev"].(string), "--append", "   "); rc != 2 {
		t.Fatal("empty append accepted")
	}
	h := mustCreate(t, d, "hunch", `{"question":"Why?"}`)
	hp := "hunches/" + h["id"].(string) + ".md"
	before := readVaultFile(t, d, hp)
	rc, _, s = runCmd(d, "update", h["id"].(string), "--rev", h["rev"].(string), "--append", "more")
	if rc != 2 || !strings.Contains(s, "append works on topics only") {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if readVaultFile(t, d, hp) != before {
		t.Fatal("hunch changed")
	}
}

func TestCRUDUpdateDelete(t *testing.T) {
	d := newVault(t)
	m := mustCreate(t, d, "topic", `{"title":"Trees","tags":["poetry"]}`)
	if !strings.Contains(readVaultFile(t, d, "topics/trees.md"), "tags:") {
		t.Fatal("tags not created")
	}
	rc, _, s := runCmd(d, "update", "trees", "--rev", m["rev"].(string), "--data", `{"tags":null}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if strings.Contains(readVaultFile(t, d, "topics/trees.md"), "tags:") {
		t.Fatal("tags not removed")
	}
}
