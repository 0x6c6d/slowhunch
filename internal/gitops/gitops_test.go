package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

func needGit(t *testing.T) {
	t.Helper()
	if !Available() {
		t.Skip("git not installed")
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	argv := append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)
	out, err := exec.Command("git", argv...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	needGit(t)
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInitRepo(t *testing.T) {
	needGit(t)
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Fatal("empty temp dir reported as repo")
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(dir) {
		t.Fatal("IsRepo false after Init")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0o755)
	if !IsRepo(sub) {
		t.Fatal("subdirectory not inside work tree")
	}
}

func TestConflictDirty(t *testing.T) {
	dir := newRepo(t)
	for _, f := range []string{"clean.md", "mod.md", "staged.md", "del.md"} {
		write(t, dir, f, f+"\n")
	}
	write(t, dir, ".gitignore", "ign.md\n")
	git(t, dir, "add", "--", "clean.md", "mod.md", "staged.md", "del.md", ".gitignore")
	git(t, dir, "commit", "-q", "-m", "init")

	write(t, dir, "mod.md", "changed\n")
	write(t, dir, "staged.md", "changed\n")
	git(t, dir, "add", "--", "staged.md")
	os.Remove(filepath.Join(dir, "del.md"))
	write(t, dir, "new/untracked.md", "u\n")
	write(t, dir, "ign.md", "i\n")

	r := New(dir)
	got, err := r.Dirty([]string{"clean.md", "mod.md", "staged.md", "del.md", "new/untracked.md", "ign.md", "missing.md"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"del.md", "mod.md", "new/untracked.md", "staged.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Dirty = %v, want %v", got, want)
	}
	got, err = r.Dirty([]string{"clean.md", "ign.md", "missing.md"})
	if err != nil || len(got) != 0 {
		t.Fatalf("Dirty clean = %v, %v", got, err)
	}
}

func TestCommitExact(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "base.md", "b\n")
	write(t, dir, "gone.md", "g\n")
	write(t, dir, "other.md", "o\n")
	git(t, dir, "add", "--", "base.md", "gone.md", "other.md")
	git(t, dir, "commit", "-q", "-m", "init")

	write(t, dir, "other.md", "unrelated modified\n")
	write(t, dir, "foreign.md", "staged\n")
	git(t, dir, "add", "--", "foreign.md")
	write(t, dir, "a.md", "a\n")
	write(t, dir, "notes/b.md", "b\n")

	r := New(dir)
	before := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))
	h, err := r.Commit([]string{"a.md", "notes/b.md"}, "slowhunch: two", "run: x")
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 40 || h != strings.TrimSpace(git(t, dir, "rev-parse", "HEAD")) {
		t.Fatalf("bad hash %q", h)
	}
	if p := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD~1")); p != before {
		t.Fatalf("expected exactly one new commit")
	}
	files := strings.Fields(git(t, dir, "show", "--name-only", "--format=", "HEAD"))
	sort.Strings(files)
	if !reflect.DeepEqual(files, []string{"a.md", "notes/b.md"}) {
		t.Fatalf("committed %v", files)
	}
	stat := git(t, dir, "show", "--stat", "--name-only", "HEAD")
	if strings.Contains(stat, "foreign.md") || strings.Contains(stat, "other.md") {
		t.Fatalf("unrelated file in commit:\n%s", stat)
	}
	if msg := git(t, dir, "log", "-1", "--format=%s%n%b"); !strings.HasPrefix(msg, "slowhunch: two\nrun: x") {
		t.Fatalf("message %q", msg)
	}
	status := git(t, dir, "status", "--porcelain=v1")
	if !strings.Contains(status, "A  foreign.md") || !strings.Contains(status, " M other.md") {
		t.Fatalf("unrelated changes lost:\n%s", status)
	}

	// deletion of a tracked file is committed when listed
	os.Remove(filepath.Join(dir, "gone.md"))
	if _, err := r.Commit([]string{"gone.md"}, "delete", ""); err != nil {
		t.Fatal(err)
	}
	if out := git(t, dir, "show", "--name-status", "--format=", "HEAD"); strings.TrimSpace(out) != "D\tgone.md" {
		t.Fatalf("deletion commit: %q", out)
	}
	status = git(t, dir, "status", "--porcelain=v1")
	if !strings.Contains(status, "A  foreign.md") || !strings.Contains(status, " M other.md") {
		t.Fatalf("unrelated changes lost after delete:\n%s", status)
	}
}

func TestCommitNothing(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.md", "a\n")
	r := New(dir)
	if _, err := r.Commit([]string{"a.md"}, "first", ""); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))
	h, err := r.Commit([]string{"a.md", "missing.md"}, "again", "")
	if err != nil || h != "" {
		t.Fatalf("Commit = %q, %v; want empty", h, err)
	}
	if now := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD")); now != head {
		t.Fatal("HEAD moved")
	}
}

func TestGitMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", "")
	if Available() {
		t.Fatal("Available true with empty PATH")
	}
	if IsRepo(dir) {
		t.Fatal("IsRepo true with empty PATH")
	}
	if err := Init(dir); errs.CodeOf(err) != errs.GitMissing {
		t.Fatalf("Init err = %v", err)
	}
	if _, err := New(dir).Dirty([]string{"a"}); errs.CodeOf(err) != errs.GitMissing {
		t.Fatalf("Dirty err = %v", err)
	}
	_, err := New(dir).Commit([]string{"a"}, "s", "b")
	if errs.CodeOf(err) != errs.GitMissing {
		t.Fatalf("Commit err = %v", err)
	}
	if errs.ExitCode(err) != 1 {
		t.Fatalf("exit code %d", errs.ExitCode(err))
	}
}
