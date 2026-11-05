#!/usr/bin/env bash
# Verification for T009 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "gitops tests" <<'CMD'
gotest "./internal/gitops/..." 'TestInitRepo|TestConflictDirty|TestCommitExact|TestCommitNothing|TestGitMissing' TestInitRepo TestConflictDirty TestCommitExact TestCommitNothing TestGitMissing
CMD
expect_ok "probe: init, dirty detection, commit returns a hash" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/0x6c6d/slowhunch/internal/gitops"
)

func main() {
	dir, _ := os.MkdirTemp("", "gprobe")
	defer os.RemoveAll(dir)
	if !gitops.Available() || gitops.Init(dir) != nil || !gitops.IsRepo(dir) {
		os.Exit(1)
	}
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("a\n"), 0o644)
	r := gitops.New(dir)
	d, err := r.Dirty([]string{"a.md", "missing.md"})
	if err != nil || len(d) != 1 || d[0] != "a.md" {
		os.Exit(1)
	}
	h, err := r.Commit([]string{"a.md"}, "slowhunch: test a (files: 1)", "run: x")
	if err != nil || len(h) < 40 {
		os.Exit(1)
	}
	out, _ := exec.Command("git", "-C", dir, "log", "--format=%s").Output()
	if string(out) != "slowhunch: test a (files: 1)\n" {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: an unrelated staged file is never committed" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/gitops"
)

func main() {
	dir, _ := os.MkdirTemp("", "gprobe")
	defer os.RemoveAll(dir)
	if gitops.Init(dir) != nil {
		os.Exit(125)
	}
	os.WriteFile(filepath.Join(dir, "foreign.md"), []byte("f\n"), 0o644)
	exec.Command("git", "-C", dir, "add", "foreign.md").Run()
	os.WriteFile(filepath.Join(dir, "mine.md"), []byte("m\n"), 0o644)
	if _, err := gitops.New(dir).Commit([]string{"mine.md"}, "s", "b"); err != nil {
		os.Exit(125)
	}
	out, _ := exec.Command("git", "-C", dir, "show", "--name-only", "--format=", "HEAD").Output()
	if strings.Contains(string(out), "foreign.md") {
		os.Exit(0) // foreign file leaked into the commit
	}
	os.Exit(1)
}
GO
CMD
expect_fail "negative: no forbidden git verbs or flags in gitops source" <<'CMD'
ls internal/gitops/*.go >/dev/null 2>&1 || exit 125
grep -nE -e '"(push|pull|fetch|reset|checkout|rebase)"' -e '"--force"' -e '"-f"' -e '"-A"' -e '"--all"' -e 'sh", "-c' $(ls internal/gitops/*.go | grep -v '_test.go$')
CMD
finish
