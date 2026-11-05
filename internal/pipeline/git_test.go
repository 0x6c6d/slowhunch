package pipeline

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/intake"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// crashCommitter fails the first fail Commit calls, as if the process died
// after writing but before committing.
type crashCommitter struct {
	inner vault.Committer
	fail  int
	calls int
}

func (c *crashCommitter) Dirty(paths []string) ([]string, error) { return c.inner.Dirty(paths) }

func (c *crashCommitter) Commit(paths []string, subject, body string) (string, error) {
	c.calls++
	if c.calls <= c.fail {
		return "", errors.New("simulated crash before commit")
	}
	return c.inner.Commit(paths, subject, body)
}

func gitStatus(t *testing.T, dir string, paths ...string) string {
	t.Helper()
	args := append([]string{"-C", dir, "status", "--porcelain=v1", "--untracked-files=all", "--"}, paths...)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimRight(string(out), "\n")
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@localhost", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func pendingOf(t *testing.T, v *vault.Vault) []string {
	t.Helper()
	st, err := v.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	return st.Pending
}

// assertUntouched checks that a refused run left no trace.
func assertUntouched(t *testing.T, v *vault.Vault, tree, head string) {
	t.Helper()
	if treeHash(t, v.Root) != tree {
		t.Fatal("vault tree changed")
	}
	if headOf(t, v.Root) != head {
		t.Fatal("git HEAD moved")
	}
	if p := pendingOf(t, v); len(p) != 0 {
		t.Fatalf("pending %v", p)
	}
	if v.Exists(".slowhunch/reports") {
		t.Fatal("report written")
	}
}

func TestConflict(t *testing.T) {
	v, r := addVault(t)
	seedHunch(t, v, r, "h1", "Does deferral in a line read as meaning?", "open")
	hp := filepath.Join(v.Root, "hunches", "h1.md")
	f, err := os.OpenFile(hp, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("hand edit\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	edited, _ := os.ReadFile(hp)
	tree, head := treeHash(t, v.Root), headOf(t, v.Root)

	rep, err := Add(context.Background(), addDeps(v, r, addFake()), Options{Arg: addArticle, Medium: "article", Title: "Flood", Hunch: "h1"})
	if errs.CodeOf(err) != errs.Conflict || rep != nil {
		t.Fatalf("got %v (report %v), want CONFLICT", err, rep)
	}
	if !strings.Contains(err.Error(), "hunches/h1.md") {
		t.Fatalf("conflict does not name the hunch: %v", err)
	}
	if errs.ExitCode(err) != 4 {
		t.Fatalf("exit code %d", errs.ExitCode(err))
	}
	assertUntouched(t, v, tree, head)
	if n := noteFiles(t, v); len(n) != 0 {
		t.Fatalf("notes written %v", n)
	}
	if b, _ := os.ReadFile(hp); string(b) != string(edited) {
		t.Fatal("hand edit lost")
	}

	// Once the edit is committed the same run goes through.
	if _, err := r.Commit([]string{"hunches/h1.md"}, "hand edit", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(context.Background(), addDeps(v, r, addFake()), Options{Arg: addArticle, Medium: "article", Title: "Flood", Hunch: "h1"}); err != nil {
		t.Fatal(err)
	}
}

func TestConflictUntrackedTarget(t *testing.T) {
	v, r := addVault(t)
	sid := vault.SourceID("article", "Ada Lovelace", "Flood", intake.Normalize(addArticle, "article"))
	if err := os.MkdirAll(filepath.Join(v.Root, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v.Root, "inbox", sid+".txt"), []byte("foreign\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, head := treeHash(t, v.Root), headOf(t, v.Root)
	f := addFake()
	rep, err := Add(context.Background(), addDeps(v, r, f), Options{Arg: addArticle, Medium: "article", Title: "Flood", Author: "Ada Lovelace"})
	if errs.CodeOf(err) != errs.Conflict || rep != nil {
		t.Fatalf("got %v (report %v), want CONFLICT", err, rep)
	}
	if !strings.Contains(err.Error(), "inbox/"+sid+".txt") {
		t.Fatalf("conflict does not name the inbox file: %v", err)
	}
	if len(f.Calls) != 0 {
		t.Fatalf("%d provider calls", len(f.Calls))
	}
	assertUntouched(t, v, tree, head)
	if v.Exists("sources/"+sid+".md") || v.Exists("sources/"+sid+"-2.md") || len(noteFiles(t, v)) != 0 {
		t.Fatal("something was written")
	}
}

func TestCommitExact(t *testing.T) {
	v, r := addVault(t)
	seedHunch(t, v, r, "h1", "Does deferral in a line read as meaning?", "open")
	if err := os.MkdirAll(filepath.Join(v.Root, "misc"), 0o755); err != nil {
		t.Fatal(err)
	}
	tracked := filepath.Join(v.Root, "misc", "tracked.txt")
	staged := filepath.Join(v.Root, "misc", "staged.txt")
	if err := os.WriteFile(tracked, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit([]string{"misc/tracked.txt"}, "seed misc", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracked, []byte("v2 uncommitted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("staged only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, v.Root, "add", "--", "misc/staged.txt")
	head := headOf(t, v.Root)

	rep, err := Add(context.Background(), addDeps(v, r, addFake()), Options{Arg: addArticle, Medium: "article", Title: "Flood", Hunch: "h1"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Git.Commit == "" || rep.Git.Commit != headOf(t, v.Root) || commitsSince(t, v.Root, head) != "1" {
		t.Fatalf("git %+v", rep.Git)
	}
	sid := unlink(rep.Source)
	want := []string{"hunches/h1.md", "inbox/" + sid + ".txt", "sources/" + sid + ".md"}
	for _, a := range rep.Atoms {
		want = append(want, "notes/"+unlink(a.ID)+".md")
	}
	sort.Strings(want)
	if got := committedFiles(t, v.Root); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("committed %v, want %v", got, want)
	}
	if st := gitStatus(t, v.Root, "misc"); st != " M misc/tracked.txt\nA  misc/staged.txt" && st != "A  misc/staged.txt\n M misc/tracked.txt" {
		t.Fatalf("unrelated files status %q", st)
	}
	if b, _ := os.ReadFile(tracked); string(b) != "v2 uncommitted\n" {
		t.Fatalf("tracked file changed: %q", b)
	}
	if b, _ := os.ReadFile(staged); string(b) != "staged only\n" {
		t.Fatalf("staged file changed: %q", b)
	}
	if st := gitStatus(t, v.Root, "inbox", "sources", "notes", "hunches"); st != "" {
		t.Fatalf("run files left uncommitted: %q", st)
	}
	if p := pendingOf(t, v); len(p) != 0 {
		t.Fatalf("pending %v", p)
	}
}

func TestResume(t *testing.T) {
	v, r := addVault(t)
	head := headOf(t, v.Root)
	cc := &crashCommitter{inner: r, fail: 1}
	d := addDeps(v, r, addFake())
	d.Committer = cc

	rep, err := Add(context.Background(), d, Options{Arg: addArticle, Medium: "article", Title: "Flood"})
	if err == nil || rep != nil {
		t.Fatalf("crashed run: %v %v", rep, err)
	}
	if headOf(t, v.Root) != head {
		t.Fatal("crashed run committed")
	}
	sid1 := vault.SourceID("article", "", "Flood", intake.Normalize(addArticle, "article"))
	pend := pendingOf(t, v)
	var first []string
	for _, p := range pend {
		if !v.Exists(p) {
			t.Fatalf("pending %s not on disk", p)
		}
		first = append(first, p)
	}
	has := map[string]bool{}
	for _, p := range pend {
		has[p] = true
	}
	if !has["inbox/"+sid1+".txt"] || !has["sources/"+sid1+".md"] || len(pend) < 3 {
		t.Fatalf("pending after crash %v", pend)
	}

	// The next run adopts the pending files without a CONFLICT and commits
	// them together with its own files in one commit.
	d2 := addDeps(v, r, addFake())
	d2.Committer = cc
	rep2, err := Add(context.Background(), d2, Options{Arg: addArticle2, Medium: "article", Title: "Pump"})
	if err != nil {
		t.Fatal(err)
	}
	if rep2.Git.Commit == "" || commitsSince(t, v.Root, head) != "1" {
		t.Fatalf("git %+v, %s commits", rep2.Git, commitsSince(t, v.Root, head))
	}
	sid2 := unlink(rep2.Source)
	want := append([]string{}, first...)
	want = append(want, "inbox/"+sid2+".txt", "sources/"+sid2+".md")
	for _, a := range rep2.Atoms {
		want = append(want, "notes/"+unlink(a.ID)+".md")
	}
	sort.Strings(want)
	if got := committedFiles(t, v.Root); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("committed %v, want %v", got, want)
	}
	if p := pendingOf(t, v); len(p) != 0 {
		t.Fatalf("pending not cleared %v", p)
	}
	if st := gitStatus(t, v.Root); st != "" {
		t.Fatalf("dirty tree after resume: %q", st)
	}
}

func TestNoCommitFlag(t *testing.T) {
	v, r := addVault(t)
	head := headOf(t, v.Root)
	rep, err := Add(context.Background(), addDeps(v, r, addFake()), Options{Arg: addArticle, Medium: "article", Title: "Flood", NoCommit: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Git.Skipped != "no-commit" || rep.Git.Commit != "" {
		t.Fatalf("git %+v", rep.Git)
	}
	if headOf(t, v.Root) != head {
		t.Fatal("--no-commit committed")
	}
	sid := unlink(rep.Source)
	if !v.Exists("inbox/"+sid+".txt") || !v.Exists("sources/"+sid+".md") || len(noteFiles(t, v)) != len(rep.Atoms) || len(rep.Atoms) == 0 {
		t.Fatalf("files not written: %+v", rep)
	}
	if !v.Exists(".slowhunch/reports/" + rep.RunID + ".json") {
		t.Fatal("report not saved")
	}
	if st := gitStatus(t, v.Root, "inbox", "sources"); !strings.Contains(st, "?? inbox/"+sid+".txt") || !strings.Contains(st, "?? sources/"+sid+".md") {
		t.Fatalf("status %q", st)
	}
}
