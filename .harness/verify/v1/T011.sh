#!/usr/bin/env bash
# Verification for T011 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "index tests" <<'CMD'
gotest "./internal/vault/..." 'TestIndexLoad|TestBacklinks|TestNeighbourhood|TestMediumOf' TestIndexLoad TestBacklinks TestNeighbourhood TestMediumOf
CMD
expect_fail "negative: malformed and misplaced files are load errors, not entries" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"path/filepath"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	dir, _ := os.MkdirTemp("", "iprobe")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755)
	v, _ := vault.Open(dir)
	v.WriteFile("topics/good.md", []byte("---\nid: good\ntype: topic\ntitle: Good\n---\nsee [[bad]]\n"))
	v.WriteFile("topics/bad.md", []byte("---\nid: bad\ntype: topic\ntitle: |\n  x\n---\n"))
	v.WriteFile("notes/wrong.md", []byte("---\nid: wrong\ntype: hunch\nquestion: q\n---\n"))
	ix, lerrs, err := v.LoadIndex()
	if err != nil {
		os.Exit(125)
	}
	if _, ok := ix.Get("good"); !ok {
		os.Exit(125)
	}
	_, okBad := ix.Get("bad")
	_, okWrong := ix.Get("wrong")
	if okBad || okWrong || len(lerrs) != 2 {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
