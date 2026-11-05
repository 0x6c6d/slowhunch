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

// isolateGit makes every git call in the test ignore user and system config.
func isolateGit(t *testing.T) {
	t.Helper()
	needGit(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
}

// remoteSetup creates a bare origin with one commit on main and two
// clones a and b tracking origin/main.
func remoteSetup(t *testing.T) (origin, a, b string) {
	t.Helper()
	isolateGit(t)
	root := t.TempDir()
	origin = filepath.Join(root, "origin.git")
	seed := filepath.Join(root, "seed")
	a = filepath.Join(root, "a")
	b = filepath.Join(root, "b")
	git(t, root, "init", "-q", "--bare", "-b", "main", origin)
	git(t, root, "init", "-q", "-b", "main", seed)
	write(t, seed, "base.md", "base\n")
	git(t, seed, "add", "base.md")
	git(t, seed, "commit", "-q", "-m", "base")
	git(t, seed, "remote", "add", "origin", origin)
	git(t, seed, "push", "-q", "origin", "main")
	git(t, root, "clone", "-q", origin, a)
	git(t, root, "clone", "-q", origin, b)
	return origin, a, b
}

func commitFile(t *testing.T, dir, name, body, msg string) {
	t.Helper()
	write(t, dir, name, body)
	git(t, dir, "add", "--", name)
	git(t, dir, "commit", "-q", "-m", msg)
}

func TestPullRebase(t *testing.T) {
	_, a, b := remoteSetup(t)
	commitFile(t, a, "a1.md", "one\n", "a1")
	commitFile(t, a, "a2.md", "two\n", "a2")
	git(t, a, "push", "-q", "origin", "main")

	commitFile(t, b, "b1.md", "local\n", "b1")
	write(t, b, "base.md", "base\nuncommitted\n")
	rb := New(b)
	ok, err := rb.HasRemote("origin")
	if err != nil || !ok {
		t.Fatalf("HasRemote origin = %v, %v", ok, err)
	}
	if ok, _ := rb.HasRemote("nope"); ok {
		t.Fatal("HasRemote nope true")
	}
	if br, err := rb.CurrentBranch(); err != nil || br != "main" {
		t.Fatalf("CurrentBranch = %q, %v", br, err)
	}
	if ok, err := rb.HasUpstream(); err != nil || !ok {
		t.Fatalf("HasUpstream = %v, %v", ok, err)
	}
	pulled, err := rb.PullRebase()
	if err != nil {
		t.Fatal(err)
	}
	if pulled != 2 {
		t.Fatalf("pulled = %d, want 2", pulled)
	}
	if merges := strings.TrimSpace(git(t, b, "rev-list", "--merges", "--count", "HEAD")); merges != "0" {
		t.Fatalf("merge commits = %s", merges)
	}
	log := strings.Fields(git(t, b, "log", "--format=%s"))
	if want := []string{"b1", "a2", "a1", "base"}; !reflect.DeepEqual(log, want) {
		t.Fatalf("history = %v, want %v", log, want)
	}
	got, err := os.ReadFile(filepath.Join(b, "base.md"))
	if err != nil || string(got) != "base\nuncommitted\n" {
		t.Fatalf("autostashed change lost: %q, %v", got, err)
	}
	if st := git(t, b, "stash", "list"); strings.TrimSpace(st) != "" {
		t.Fatalf("stash left behind: %s", st)
	}
	if n, err := rb.PullRebase(); err != nil || n != 0 {
		t.Fatalf("second pull = %d, %v", n, err)
	}

	// A fresh local branch without upstream: callers check HasUpstream.
	git(t, b, "checkout", "-q", "-b", "solo")
	if ok, err := rb.HasUpstream(); err != nil || ok {
		t.Fatalf("HasUpstream on solo = %v, %v", ok, err)
	}
}

func TestPushPlain(t *testing.T) {
	origin, a, _ := remoteSetup(t)
	commitFile(t, a, "p.md", "p\n", "p")
	ra := New(a)
	head, err := ra.Head()
	if err != nil {
		t.Fatal(err)
	}
	if err := ra.Push("origin", "main"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(git(t, origin, "rev-parse", "main")); got != head {
		t.Fatalf("origin main = %s, want %s", got, head)
	}
	for _, c := range [][2]string{{"-x", "main"}, {"origin", "-x"}, {"", "main"}, {"origin", ""}, {"origin", "+main"}, {"+origin", "main"}} {
		if err := ra.Push(c[0], c[1]); errs.CodeOf(err) != errs.BadInput {
			t.Fatalf("Push(%q, %q) err = %v, want BAD_INPUT", c[0], c[1], err)
		}
	}
}

func TestPushFailed(t *testing.T) {
	_, a, b := remoteSetup(t)
	commitFile(t, a, "a.md", "a\n", "a")
	git(t, a, "push", "-q", "origin", "main")
	commitFile(t, b, "b.md", "b\n", "b")
	rb := New(b)
	head, _ := rb.Head()
	err := rb.Push("origin", "main")
	if errs.CodeOf(err) != errs.PushFailed {
		t.Fatalf("err = %v, want PUSH_FAILED", err)
	}
	if errs.ExitCode(err) != 1 {
		t.Fatalf("exit code %d", errs.ExitCode(err))
	}
	if now, _ := rb.Head(); now != head {
		t.Fatal("commit not kept")
	}
	if err := New(b).Push("nosuchremote", "main"); errs.CodeOf(err) != errs.PushFailed {
		t.Fatalf("missing remote err = %v", err)
	}
}

func TestBehindCount(t *testing.T) {
	_, a, b := remoteSetup(t)
	commitFile(t, a, "a.md", "a\n", "a")
	git(t, a, "push", "-q", "origin", "main")
	rb := New(b)
	if n, err := rb.Behind(); err != nil || n != 0 {
		t.Fatalf("Behind before fetch = %d, %v", n, err)
	}
	git(t, b, "fetch", "-q", "origin")
	if n, err := rb.Behind(); err != nil || n != 1 {
		t.Fatalf("Behind = %d, %v, want 1", n, err)
	}
	git(t, b, "checkout", "-q", "-b", "solo")
	if n, err := rb.Behind(); err != nil || n != 0 {
		t.Fatalf("Behind without upstream = %d, %v", n, err)
	}
}

func TestRebaseConflictAborts(t *testing.T) {
	_, a, b := remoteSetup(t)
	commitFile(t, a, "base.md", "from a\n", "a edit")
	git(t, a, "push", "-q", "origin", "main")
	commitFile(t, b, "base.md", "from b\n", "b edit")
	write(t, b, "untracked.md", "keep\n")
	rb := New(b)
	head, _ := rb.Head()
	status := git(t, b, "status", "--porcelain")

	_, err := rb.PullRebase()
	if errs.CodeOf(err) != errs.Conflict {
		t.Fatalf("err = %v, want CONFLICT", err)
	}
	if errs.ExitCode(err) != 4 {
		t.Fatalf("exit code %d", errs.ExitCode(err))
	}
	e := err.(*errs.Error)
	if !strings.Contains(e.Message, "base.md") || strings.Contains(e.Message, "restore") {
		t.Fatalf("message = %q", e.Message)
	}
	if paths, _ := e.Extra["paths"].([]string); !reflect.DeepEqual(paths, []string{"base.md"}) {
		t.Fatalf("paths = %v", e.Extra["paths"])
	}
	gitDir := strings.TrimSpace(git(t, b, "rev-parse", "--absolute-git-dir"))
	for _, d := range []string{"rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(gitDir, d)); err == nil {
			t.Fatalf("rebase still in progress (%s)", d)
		}
	}
	if now, _ := rb.Head(); now != head {
		t.Fatal("HEAD moved")
	}
	if now := git(t, b, "status", "--porcelain"); now != status {
		t.Fatalf("status = %q, want %q", now, status)
	}
	if got, _ := os.ReadFile(filepath.Join(b, "base.md")); string(got) != "from b\n" {
		t.Fatalf("base.md = %q", got)
	}
}

func TestPullFailedUnreachable(t *testing.T) {
	_, _, b := remoteSetup(t)
	git(t, b, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	rb := New(b)
	head, _ := rb.Head()
	_, err := rb.PullRebase()
	if errs.CodeOf(err) != errs.PullFailed {
		t.Fatalf("err = %v, want PULL_FAILED", err)
	}
	if e := err.(*errs.Error); e.Message == "" || e.Hint != "check git.remote and your network" {
		t.Fatalf("error = %+v", e)
	}
	if now, _ := rb.Head(); now != head {
		t.Fatal("HEAD moved")
	}
}

func TestAheadCount(t *testing.T) {
	_, a, _ := remoteSetup(t)
	ra := New(a)
	if n, err := ra.Ahead(); err != nil || n != 0 {
		t.Fatalf("Ahead before commit = %d, %v", n, err)
	}
	commitFile(t, a, "a.md", "a\n", "a")
	commitFile(t, a, "b.md", "b\n", "b")
	if n, err := ra.Ahead(); err != nil || n != 2 {
		t.Fatalf("Ahead = %d, %v, want 2", n, err)
	}
	git(t, a, "push", "-q", "origin", "main")
	if n, err := ra.Ahead(); err != nil || n != 0 {
		t.Fatalf("Ahead after push = %d, %v", n, err)
	}
	git(t, a, "checkout", "-q", "-b", "solo")
	commitFile(t, a, "c.md", "c\n", "c")
	if n, err := ra.Ahead(); err != nil || n != 0 {
		t.Fatalf("Ahead without upstream = %d, %v", n, err)
	}
}
