package vault

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

func newVault(t *testing.T) *Vault {
	t.Helper()
	root := filepath.Join(t.TempDir(), "v")
	if err := os.MkdirAll(filepath.Join(root, ".slowhunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestWriteConfinement(t *testing.T) {
	v := newVault(t)
	base := filepath.Dir(v.Root)
	out := filepath.Join(base, "outside")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(out, filepath.Join(v.Root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(v.Root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"../x", "../outside/x", "/etc/x", "a/../../x", "link/x", "link", ".git/config", ".git", "", "a\x00b", "."} {
		err := v.WriteFile(rel, []byte("x"))
		if errs.CodeOf(err) != errs.BadInput {
			t.Errorf("%q: got %v, want BAD_INPUT", rel, err)
		}
		if _, err := v.PutDoc(rel, &frontmatter.Doc{}, ""); err == nil {
			t.Errorf("PutDoc %q accepted", rel)
		}
	}
	if err := v.Move("a.txt", "../x"); errs.CodeOf(err) != errs.BadInput {
		t.Errorf("move out: %v", err)
	}
	if ents, _ := os.ReadDir(out); len(ents) != 0 {
		t.Errorf("files created outside: %v", ents)
	}
	if _, err := os.Stat(filepath.Join(base, "x")); err == nil {
		t.Error("x created outside vault")
	}
	if _, err := os.Stat(filepath.Join(v.Root, ".git", "config")); err == nil {
		t.Error(".git/config written")
	}
	// A symlink inside the vault pointing inside is fine.
	if err := os.Symlink(filepath.Join(v.Root, "notes"), filepath.Join(v.Root, "inlink")); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteFile("notes/ok.md", []byte("ok")); err != nil {
		t.Fatal(err)
	}
	if b, err := v.ReadFile("inlink/ok.md"); err != nil || string(b) != "ok" {
		t.Errorf("inner symlink read: %q %v", b, err)
	}
}

func TestAtomicWrite(t *testing.T) {
	v := newVault(t)
	if err := v.WriteFile("notes/deep/a.md", []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteFile("notes/deep/a.md", []byte("two")); err != nil {
		t.Fatal(err)
	}
	b, err := v.ReadFile("notes/deep/a.md")
	if err != nil || string(b) != "two" {
		t.Fatalf("got %q %v", b, err)
	}
	ents, _ := os.ReadDir(filepath.Join(v.Root, "notes", "deep"))
	if len(ents) != 1 || ents[0].Name() != "a.md" {
		t.Errorf("leftover files: %v", ents)
	}
	if _, err := v.ReadFile("nope.md"); errs.CodeOf(err) != errs.NotFound {
		t.Errorf("missing read: %v", err)
	}
	if !v.Exists("notes/deep/a.md") || v.Exists("nope.md") {
		t.Error("Exists")
	}
	if err := v.WriteFile("b.md", []byte("b")); err != nil {
		t.Fatal(err)
	}
	if err := v.Move("b.md", "notes/deep/a.md"); errs.CodeOf(err) != errs.Exists {
		t.Errorf("move onto existing: %v", err)
	}
	if err := v.Move("b.md", "archive/b.md"); err != nil || !v.Exists("archive/b.md") || v.Exists("b.md") {
		t.Errorf("move: %v", err)
	}
	if err := v.Remove("archive/b.md"); err != nil || v.Exists("archive/b.md") {
		t.Errorf("remove: %v", err)
	}
	if err := v.Remove("archive/b.md"); errs.CodeOf(err) != errs.NotFound {
		t.Errorf("remove missing: %v", err)
	}
}

func TestConflictRev(t *testing.T) {
	v := newVault(t)
	d := &frontmatter.Doc{Body: "b\n"}
	d.Set("type", "topic")
	d.Set("id", "t-1")
	r1, err := v.PutDoc("topics/t-1.md", d, "")
	if err != nil || len(r1) != 6 {
		t.Fatalf("create: %q %v", r1, err)
	}
	if _, err := v.PutDoc("topics/t-1.md", d, ""); errs.CodeOf(err) != errs.Exists {
		t.Errorf("create over existing: %v", err)
	}
	cur, _ := v.ReadFile("topics/t-1.md")
	stored, _ := frontmatter.Parse(cur)
	if stored.GetString("rev") != r1 {
		t.Errorf("stored rev %q want %q", stored.GetString("rev"), r1)
	}
	d.Body = "changed\n"
	_, err = v.PutDoc("topics/t-1.md", d, "000000")
	if errs.CodeOf(err) != errs.ConflictRev {
		t.Fatalf("stale: %v", err)
	}
	e := err.(*errs.Error)
	if e.Extra["current"] != string(cur) || e.Extra["rev"] != r1 {
		t.Errorf("extra: %v", e.Extra)
	}
	after, _ := v.ReadFile("topics/t-1.md")
	if string(after) != string(cur) {
		t.Error("stale write modified file")
	}
	r2, err := v.PutDoc("topics/t-1.md", d, r1)
	if err != nil || r2 == r1 || len(r2) != 6 {
		t.Fatalf("update: %q %v", r2, err)
	}
	if b, _ := v.ReadFile("topics/t-1.md"); !strings.Contains(string(b), "changed") {
		t.Error("update not written")
	}
}

func TestEvidenceImmutable(t *testing.T) {
	v := newVault(t)
	d := &frontmatter.Doc{}
	d.Set("type", "note")
	d.Set("id", "n-1")
	d.Set("evidence", "original")
	r1, err := v.PutDoc("notes/n-1.md", d, "")
	if err != nil {
		t.Fatal(err)
	}
	d2 := d.Clone()
	d2.Set("evidence", "rewritten")
	if _, err := v.PutDoc("notes/n-1.md", d2, r1); errs.CodeOf(err) != errs.BadInput {
		t.Errorf("evidence rewrite: %v", err)
	}
	d3 := d.Clone()
	d3.Set("confidence", "0.7")
	if _, err := v.PutDoc("notes/n-1.md", d3, r1); err != nil {
		t.Errorf("other change: %v", err)
	}
}

func TestInboxImmutable(t *testing.T) {
	v := newVault(t)
	if err := v.WriteFile("inbox/s-1.txt", []byte("raw")); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteFile("inbox/s-1.txt", []byte("new")); errs.CodeOf(err) != errs.Exists {
		t.Errorf("overwrite inbox: %v", err)
	}
	if b, _ := v.ReadFile("inbox/s-1.txt"); string(b) != "raw" {
		t.Errorf("inbox changed: %q", b)
	}
}

func TestTestGuard(t *testing.T) {
	prot := t.TempDir()
	sib := t.TempDir()
	for _, d := range []string{prot, filepath.Join(prot, "sub"), sib} {
		if err := os.MkdirAll(filepath.Join(d, ".slowhunch"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SLOWHUNCH_TEST", "1")
	t.Setenv("SLOWHUNCH_PROTECTED", "/nonexistent-slowhunch"+string(os.PathListSeparator)+prot)
	for _, p := range []string{prot, filepath.Join(prot, "sub"), filepath.Join(prot, "sub", "new")} {
		if err := GuardPath(p); errs.CodeOf(err) != errs.BadInput {
			t.Errorf("GuardPath %s: %v", p, err)
		}
	}
	if _, err := Open(prot); errs.CodeOf(err) != errs.BadInput {
		t.Errorf("Open protected: %v", err)
	}
	if _, err := Open(filepath.Join(prot, "sub")); errs.CodeOf(err) != errs.BadInput {
		t.Errorf("Open protected sub: %v", err)
	}
	if err := GuardPath(sib); err != nil {
		t.Errorf("sibling refused: %v", err)
	}
	if err := GuardPath(prot + "x"); err != nil {
		t.Errorf("prefix sibling refused: %v", err)
	}
	if _, err := Open(sib); err != nil {
		t.Errorf("Open sibling: %v", err)
	}
	if _, err := Open(t.TempDir()); errs.CodeOf(err) != errs.NotFound {
		t.Errorf("Open non-vault: %v", err)
	}
	t.Setenv("SLOWHUNCH_TEST", "")
	if err := GuardPath(prot); err != nil {
		t.Errorf("guard active without SLOWHUNCH_TEST: %v", err)
	}
}

func TestAuditAndState(t *testing.T) {
	v := newVault(t)
	s, err := v.LoadState()
	if err != nil || s.PromptVersions != nil || s.Pending != nil {
		t.Fatalf("zero state: %+v %v", s, err)
	}
	want := State{PromptVersions: map[string]string{"extract": "v1"}, Pending: []string{"inbox/a.txt"}}
	if err := v.SaveState(want); err != nil {
		t.Fatal(err)
	}
	got, err := v.LoadState()
	if err != nil || got.PromptVersions["extract"] != "v1" || len(got.Pending) != 1 || got.Pending[0] != "inbox/a.txt" {
		t.Errorf("state round trip: %+v %v", got, err)
	}
	for i, cmd := range []string{"add", "distill"} {
		e := AuditEntry{Time: "2026-11-05T00:00:00Z", RunID: "r" + string(rune('0'+i)), Command: cmd,
			Paths: []string{"notes/a.md"}, PromptVersions: map[string]string{"x": "1"}, Commit: "abc"}
		if err := v.Audit(e); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open(filepath.Join(v.Root, ".slowhunch", "audit.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var n int
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("line %d: %v", n, err)
		}
		for _, k := range []string{"time", "run_id", "command", "paths", "prompt_versions", "commit"} {
			if _, ok := m[k]; !ok {
				t.Errorf("line %d missing %s", n, k)
			}
		}
		n++
	}
	if n != 2 {
		t.Errorf("audit lines: %d", n)
	}
}
