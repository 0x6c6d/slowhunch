package vault_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func needGit(t *testing.T) {
	t.Helper()
	if !gitops.Available() {
		t.Skip("git not available")
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeRaw(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitVault creates a vault with a git repo and one seed commit containing
// hunches/h1.md and notes/other.md.
func gitVault(t *testing.T) (*vault.Vault, *gitops.Repo) {
	t.Helper()
	needGit(t)
	root := filepath.Join(t.TempDir(), "v")
	if err := os.MkdirAll(filepath.Join(root, ".slowhunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := gitops.Init(root); err != nil {
		t.Fatal(err)
	}
	writeRaw(t, root, ".gitignore", ".slowhunch/state.json\n.slowhunch/audit.log\n.slowhunch/reports/\n")
	writeRaw(t, root, "hunches/h1.md", "old\n")
	writeRaw(t, root, "notes/other.md", "other\n")
	v, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	r := gitops.New(v.Root)
	if _, err := r.Commit([]string{".gitignore", "hunches/h1.md", "notes/other.md"}, "seed", ""); err != nil {
		t.Fatal(err)
	}
	return v, r
}

func commitCount(t *testing.T, dir string) string {
	return strings.TrimSpace(gitOut(t, dir, "rev-list", "--count", "HEAD"))
}

func headFiles(t *testing.T, dir string) []string {
	out := strings.Fields(gitOut(t, dir, "show", "--no-renames", "--name-only", "--format=", "HEAD"))
	sort.Strings(out)
	return out
}

func TestRunID(t *testing.T) {
	now := time.Date(2026, 11, 5, 9, 8, 7, 0, time.UTC)
	id := vault.NewRunID(now)
	if !regexp.MustCompile(`^20261105-090807-[0-9a-f]{6}$`).MatchString(id) {
		t.Fatalf("run id %q", id)
	}
	if vault.NewRunID(now) == id && vault.NewRunID(now) == id {
		t.Fatal("run ids not random")
	}
	v := func() *vault.Vault {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, ".slowhunch"), 0o755)
		v, err := vault.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}()
	tx, err := v.Begin("x", vault.TxnOptions{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tx.RunID(), "20261105-090807-") {
		t.Fatalf("txn run id %q", tx.RunID())
	}
	tx, _ = v.Begin("x", vault.TxnOptions{RunID: "given"})
	if tx.RunID() != "given" {
		t.Fatal("explicit run id ignored")
	}
}

func TestTxnConflict(t *testing.T) {
	v, r := gitVault(t)
	writeRaw(t, v.Root, "hunches/h1.md", "hand edit\n")
	writeRaw(t, v.Root, "notes/other.md", "unrelated edit\n")

	tx, err := v.Begin("add", vault.TxnOptions{Committer: r})
	if err != nil {
		t.Fatal(err)
	}
	err = tx.Touch("notes/n1.md", "hunches/h1.md")
	if errs.CodeOf(err) != errs.Conflict || errs.ExitCode(err) != 4 {
		t.Fatalf("Touch err = %v", err)
	}
	var e *errs.Error
	e = err.(*errs.Error)
	if !reflect.DeepEqual(e.Extra["paths"], []string{"hunches/h1.md"}) {
		t.Fatalf("extra paths %v", e.Extra["paths"])
	}
	if v.Exists("notes/n1.md") || v.Exists(".slowhunch/state.json") {
		t.Fatal("something was written before the conflict")
	}
	// The automatic touch in WriteFile also refuses.
	if err := tx.WriteFile("hunches/h1.md", []byte("x\n")); errs.CodeOf(err) != errs.Conflict {
		t.Fatalf("WriteFile err = %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(v.Root, "hunches", "h1.md"))
	if string(b) != "hand edit\n" {
		t.Fatal("dirty file overwritten")
	}
	// Untouched dirty files do not matter.
	if err := tx.WriteFile("notes/n1.md", []byte("n1\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Commit([]string{"n1"}, "created: 1"); err != nil {
		t.Fatal(err)
	}
	if got := headFiles(t, v.Root); !reflect.DeepEqual(got, []string{"notes/n1.md"}) {
		t.Fatalf("committed %v", got)
	}
}

func TestTxnCommitExact(t *testing.T) {
	v, r := gitVault(t)
	writeRaw(t, v.Root, "notes/other.md", "unrelated edit\n")
	tx, err := v.Begin("create", vault.TxnOptions{Committer: r,
		PromptVersions: map[string]string{"place": "3", "gate": "1", "extract": "2"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.WriteFile("topics/a.md", []byte("a\n")); err != nil {
		t.Fatal(err)
	}
	d, _ := frontmatter.Parse([]byte("---\ntype: topic\n---\nbody\n"))
	if _, err := tx.PutDoc("topics/b.md", d, ""); err != nil {
		t.Fatal(err)
	}
	if err := tx.Move("hunches/h1.md", "hunches/h2.md"); err != nil {
		t.Fatal(err)
	}
	if err := tx.WriteFile(".slowhunch/reports/r.md", []byte("r\n")); err != nil {
		t.Fatal(err)
	}
	ids := []string{"a", "b", "c", "d", "e", "f", "g"}
	h, err := tx.Commit(ids, "created: 2")
	if err != nil || h == "" {
		t.Fatalf("commit %q %v", h, err)
	}
	if commitCount(t, v.Root) != "2" {
		t.Fatal("expected exactly one new commit")
	}
	want := []string{"hunches/h1.md", "hunches/h2.md", "topics/a.md", "topics/b.md"}
	if got := headFiles(t, v.Root); !reflect.DeepEqual(got, want) {
		t.Fatalf("committed %v want %v", got, want)
	}
	msg := gitOut(t, v.Root, "log", "-1", "--format=%B")
	lines := strings.Split(strings.TrimSpace(msg), "\n")
	if lines[0] != "slowhunch: create a b c d e +2 more (created: 2)" {
		t.Fatalf("subject %q", lines[0])
	}
	if !strings.Contains(msg, "run: "+tx.RunID()+"\n") || !strings.Contains(msg, "prompts: extract=2, gate=1, place=3") {
		t.Fatalf("body %q", msg)
	}
	if st := gitOut(t, v.Root, "status", "--porcelain", "--", "notes/other.md"); !strings.HasPrefix(st, " M") {
		t.Fatalf("unrelated file status %q", st)
	}
	s, _ := v.LoadState()
	if len(s.Pending) != 0 {
		t.Fatalf("pending %v", s.Pending)
	}
	audit, _ := os.ReadFile(filepath.Join(v.Root, ".slowhunch", "audit.log"))
	if !strings.Contains(string(audit), h) || !strings.Contains(string(audit), tx.RunID()) {
		t.Fatalf("audit %s", audit)
	}
	if _, err := tx.Commit(nil, ""); err == nil {
		t.Fatal("second commit accepted")
	}
}

func TestTxnResume(t *testing.T) {
	v, r := gitVault(t)
	tx, err := v.Begin("add", vault.TxnOptions{Committer: r})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.WriteFile("notes/a.md", []byte("a\n")); err != nil {
		t.Fatal(err)
	}
	if err := tx.WriteFile("hunches/h1.md", []byte("changed\n")); err != nil {
		t.Fatal(err)
	}
	s, _ := v.LoadState()
	if !reflect.DeepEqual(s.Pending, []string{"hunches/h1.md", "notes/a.md"}) {
		t.Fatalf("pending after crash %v", s.Pending)
	}
	tx = nil // crash: no Commit

	tx2, err := v.Begin("add", vault.TxnOptions{Committer: r})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx2.Touch("notes/a.md", "hunches/h1.md"); err != nil {
		t.Fatalf("pending files reported as conflict: %v", err)
	}
	if err := tx2.WriteFile("notes/c.md", []byte("c\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := tx2.Commit([]string{"c"}, "created: 1"); err != nil {
		t.Fatal(err)
	}
	if commitCount(t, v.Root) != "2" {
		t.Fatal("expected one commit")
	}
	want := []string{"hunches/h1.md", "notes/a.md", "notes/c.md"}
	if got := headFiles(t, v.Root); !reflect.DeepEqual(got, want) {
		t.Fatalf("committed %v", got)
	}
	s, _ = v.LoadState()
	if len(s.Pending) != 0 {
		t.Fatalf("pending not cleared %v", s.Pending)
	}
}

func treeHash(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		h.Write([]byte(rel + "\x00"))
		if !d.IsDir() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h.Write(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestTxnDryRun(t *testing.T) {
	v, r := gitVault(t)
	d0, _ := frontmatter.Parse([]byte("---\ntype: topic\n---\nbody\n"))
	tx0, _ := v.Begin("seed", vault.TxnOptions{Committer: r})
	rev0, err := tx0.PutDoc("topics/x.md", d0, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx0.Commit(nil, ""); err != nil {
		t.Fatal(err)
	}
	before := treeHash(t, v.Root)
	count := commitCount(t, v.Root)

	tx, err := v.Begin("create", vault.TxnOptions{Committer: r, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.WriteFile("topics/a.md", []byte("a\n")); err != nil {
		t.Fatal(err)
	}
	if err := tx.Move("hunches/h1.md", "hunches/h2.md"); err != nil {
		t.Fatal(err)
	}
	d, _ := frontmatter.Parse([]byte("---\ntype: topic\n---\nnew\n"))
	if _, err := tx.PutDoc("topics/x.md", d, ""); errs.CodeOf(err) != errs.Exists {
		t.Fatalf("dry-run EXISTS check: %v", err)
	}
	if _, err := tx.PutDoc("topics/x.md", d, "stale"); errs.CodeOf(err) != errs.ConflictRev {
		t.Fatalf("dry-run CONFLICT_REV check: %v", err)
	}
	rev, err := tx.PutDoc("topics/x.md", d, rev0)
	if err != nil || rev == "" || rev == rev0 {
		t.Fatalf("dry-run rev %q %v", rev, err)
	}
	want := []string{"hunches/h1.md", "hunches/h2.md", "topics/a.md", "topics/x.md"}
	if !reflect.DeepEqual(tx.Written(), want) {
		t.Fatalf("written %v", tx.Written())
	}
	h, err := tx.Commit([]string{"a"}, "created: 1")
	if err != nil || h != "" {
		t.Fatalf("dry-run commit %q %v", h, err)
	}
	if treeHash(t, v.Root) != before {
		t.Fatal("dry-run changed the vault")
	}
	if commitCount(t, v.Root) != count {
		t.Fatal("dry-run made a commit")
	}
}

func TestTxnNoCommit(t *testing.T) {
	for _, name := range []string{"no-commit", "no-git"} {
		t.Run(name, func(t *testing.T) {
			v, r := gitVault(t)
			o := vault.TxnOptions{Committer: r, NoCommit: true}
			if name == "no-git" {
				o = vault.TxnOptions{}
			}
			writeRaw(t, v.Root, "hunches/h1.md", "hand edit\n")
			tx, err := v.Begin("add", o)
			if err != nil {
				t.Fatal(err)
			}
			// With commits disabled there is no conflict check.
			if err := tx.WriteFile("hunches/h1.md", []byte("tool\n")); err != nil {
				t.Fatal(err)
			}
			if err := tx.WriteFile("notes/n.md", []byte("n\n")); err != nil {
				t.Fatal(err)
			}
			h, err := tx.Commit([]string{"n"}, "created: 1")
			if err != nil || h != "" {
				t.Fatalf("commit %q %v", h, err)
			}
			if commitCount(t, v.Root) != "1" {
				t.Fatal("commit made despite no-commit")
			}
			s, _ := v.LoadState()
			if len(s.Pending) != 0 {
				t.Fatalf("pending %v", s.Pending)
			}
			audit, _ := os.ReadFile(filepath.Join(v.Root, ".slowhunch", "audit.log"))
			if !strings.Contains(string(audit), `"commit":""`) || !strings.Contains(string(audit), "notes/n.md") {
				t.Fatalf("audit %s", audit)
			}
		})
	}
}
