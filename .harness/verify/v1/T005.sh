#!/usr/bin/env bash
# Verification for T005 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "frontmatter tests" <<'CMD'
gotest "./internal/frontmatter/..." 'TestRoundTrip|TestRev|TestRejects|TestLineOf' TestRoundTrip TestRevStable TestRevChanges TestRejects TestLineOf
CMD
expect_ok "probe: quoted scalars, lists, maps round-trip; rev is 6 hex and stable" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"regexp"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

func main() {
	d := &frontmatter.Doc{Body: "Claim line\n\n> a: b\n"}
	d.Set("id", "x-1")
	d.Set("claim", "has: colon # and \"quotes\"\nand a newline")
	d.Set("tags", []string{})
	d.Set("bears_on", []string{"h1", "needs: quote"})
	d.Set("evidence", []map[string]string{{"relation": "supports", "date": "2026-11-05", "note": "x-1"}})
	b := d.Bytes()
	p, err := frontmatter.Parse(b)
	if err != nil || string(p.Bytes()) != string(b) {
		os.Exit(1)
	}
	if p.GetString("claim") != "has: colon # and \"quotes\"\nand a newline" || len(p.GetList("bears_on")) != 2 || p.GetMaps("evidence")[0]["note"] != "x-1" {
		os.Exit(1)
	}
	r := frontmatter.Rev(p)
	p.Set("rev", r)
	if !regexp.MustCompile(`^[0-9a-f]{6}$`).MatchString(r) || frontmatter.Rev(p) != r {
		os.Exit(1)
	}
	p.Set("claim", "changed")
	if frontmatter.Rev(p) == r {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: block scalar, duplicate key, flow map, missing close are rejected" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

func main() {
	bad := []string{
		"---\na: |\n  x\n---\n",
		"---\na: x\na: y\n---\n",
		"---\na: {b: c}\n---\n",
		"---\na: x\n",
		"---\na: &anchor x\n---\n",
		"---\n\ta: x\n---\n",
	}
	for _, s := range bad {
		if _, err := frontmatter.Parse([]byte(s)); err == nil {
			os.Exit(0) // accepted invalid input: the negative check must fail
		}
	}
	os.Exit(1) // all rejected
}
GO
CMD
expect_ok "no third-party imports" <<'CMD'
! grep -rhoE '"[a-z0-9.-]+\.[a-z]+/[^"]*"' internal/frontmatter | grep -v '"github.com/0x6c6d/slowhunch/'
CMD
finish
