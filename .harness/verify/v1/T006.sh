#!/usr/bin/env bash
# Verification for T006 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "id tests" <<'CMD'
gotest "./internal/vault/..." 'TestIDExamples|TestUniqueID|TestSlugSafety' TestIDExamples TestUniqueID TestSlugSafety
CMD
expect_ok "probe: spec id examples and -2 collisions" <<'CMD'
goprobe <<'GO'
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	t := "The trees are coming into leaf\nLike something almost being said;"
	sum := sha256.Sum256([]byte(t))
	short := hex.EncodeToString(sum[:])[:4]
	if vault.ShortHash(t) != short || vault.SourceID("poem", "Philip Larkin", "Trees", t) != "poem-larkin-trees-"+short {
		os.Exit(1)
	}
	if vault.AtomID("crafts", "poem-larkin-trees-ab12", "lines 1-2", "") != "crafts-larkin-trees-lines-1-2" {
		os.Exit(1)
	}
	if vault.SourceID("quote", "", "Hi", t) != "quote-anonymous-hi-"+short || vault.SourceID("own", "Me", "My Notes", t) != "own-my-notes-"+short {
		os.Exit(1)
	}
	taken := map[string]bool{"a": true, "a-2": true}
	if vault.UniqueID("a", func(s string) bool { return taken[s] }) != "a-3" || vault.UniqueID("b", func(s string) bool { return taken[s] }) != "b" {
		os.Exit(1)
	}
	if vault.KebabID("What makes a line stick?") != "what-makes-a-line-stick" {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: hostile inputs never yield unsafe slug characters" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"regexp"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	ok := regexp.MustCompile(`^([a-z0-9]+(-[a-z0-9]+)*)?$`)
	in := []string{"../../etc/passwd", "/abs/path", "a\x00b", `C:\x\..`, "..", "%2e%2e/", "a/../../b"}
	for _, s := range in {
		outs := []string{vault.Slug(s, 24), vault.KebabID(s), vault.SourceID("poem", s, s, s), vault.AtomID("reads", vault.SourceID("poem", s, s, s), s, s)}
		for _, o := range outs {
			if !ok.MatchString(o) {
				os.Exit(0) // unsafe output got through: the negative check must fail
			}
		}
	}
	os.Exit(1) // all sanitized
}
GO
CMD
finish
