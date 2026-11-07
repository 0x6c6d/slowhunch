#!/usr/bin/env bash
# Verification for T001 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "wikilink unit tests and old normalize tests" <<'CMD'
gotest ./internal/intake/ 'Wikilink|Normalize|TextHash' TestWikilinkStrip TestWikilinkCodeUntouched TestWikilinkHash TestNormalizeTable TestNormalizeIdempotent TestTextHash
CMD
expect_ok "probe: strips only brackets, idempotent" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/intake"
)

func main() {
	got := intake.Normalize("x [[a|b]] [t](u) [[]] ![[c]] [[[[d]]]]", "article")
	if got != "x a|b [t](u) [[]] c d" || intake.Normalize(got, "article") != got {
		println(got)
		os.Exit(1)
	}
	if intake.Normalize("m[[i]] = 1", "code") != "m[[i]] = 1" {
		os.Exit(1)
	}
}
GO
CMD
expect_fail "negative: probe must not accept unstripped output" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/intake"
)

func main() {
	if intake.Normalize("see [[a]]", "article") == "see [[a]]" {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
expect_ok "make check" <<'CMD'
make check
CMD
finish
