#!/usr/bin/env bash
# Verification for T010 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "txn tests" <<'CMD'
gotest "./internal/vault/..." 'TestTxn|TestRunID' TestTxnConflict TestTxnCommitExact TestTxnResume TestTxnDryRun TestTxnNoCommit TestRunID
CMD
expect_ok "vault package does not import gitops (non-test files)" <<'CMD'
! grep -l 'internal/gitops' $(ls internal/vault/*.go | grep -v '_test.go$')
CMD
expect_ok "probe: one commit with exact subject, run id format" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	dir, _ := os.MkdirTemp("", "tprobe")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755)
	gitops.Init(dir)
	v, _ := vault.Open(dir)
	if !regexp.MustCompile(`^\d{8}-\d{6}-[0-9a-f]{6}$`).MatchString(vault.NewRunID(time.Now())) {
		os.Exit(1)
	}
	t, err := v.Begin("create", vault.TxnOptions{Committer: gitops.New(v.Root)})
	if err != nil || t.WriteFile("topics/a.md", []byte("a\n")) != nil {
		os.Exit(1)
	}
	h, err := t.Commit([]string{"a"}, "created: 1")
	if err != nil || h == "" {
		os.Exit(1)
	}
	out, _ := exec.Command("git", "-C", dir, "log", "--format=%s").Output()
	if string(out) != "slowhunch: create a (created: 1)\n" {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: touching a dirty foreign file is a CONFLICT and writes nothing" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	dir, _ := os.MkdirTemp("", "tprobe")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755)
	os.MkdirAll(filepath.Join(dir, "hunches"), 0o755)
	gitops.Init(dir)
	h := filepath.Join(dir, "hunches", "h1.md")
	os.WriteFile(h, []byte("old\n"), 0o644)
	if _, err := gitops.New(dir).Commit([]string{"hunches/h1.md"}, "seed", "b"); err != nil {
		os.Exit(125)
	}
	os.WriteFile(h, []byte("hand edit\n"), 0o644)
	v, _ := vault.Open(dir)
	t, err := v.Begin("add", vault.TxnOptions{Committer: gitops.New(v.Root)})
	if err != nil {
		os.Exit(125)
	}
	err = t.Touch("notes/n1.md", "hunches/h1.md")
	_, statErr := os.Stat(filepath.Join(dir, "notes", "n1.md"))
	b, _ := os.ReadFile(h)
	out, _ := exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD").Output()
	if err == nil || errs.CodeOf(err) != errs.Conflict || statErr == nil || string(b) != "hand edit\n" || string(out) != "1\n" {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
