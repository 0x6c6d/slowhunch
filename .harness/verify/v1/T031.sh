#!/usr/bin/env bash
# Verification for T031 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "normalization tests" <<'CMD'
gotest "./internal/intake/..." 'TestNormalize|TestTextHash' TestNormalizeTable TestNormalizeIdempotent TestTextHash
CMD
expect_ok "probe: poem normalization and hash" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/intake"
)

func main() {
	in := "\r\n  The trees  are coming into leaf\r\nLike something “almost” being said—\r\n\r\n\r\n"
	want := "The trees are coming into leaf\nLike something \"almost\" being said--"
	if intake.Normalize(in, "poem") != want {
		os.Exit(1)
	}
	if intake.TextHash("abc") != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: code indentation is never collapsed; normalization is idempotent" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/intake"
)

func main() {
	code := "func f() {\n\tif x  {\n\t\treturn “y”\n\t}\n}"
	if intake.Normalize(code, "code") != code {
		os.Exit(0)
	}
	for _, s := range []string{"a—b\n\n\n\nc  d", "  x\r\n\ty "} {
		once := intake.Normalize(s, "article")
		if intake.Normalize(once, "article") != once {
			os.Exit(0)
		}
	}
	os.Exit(1)
}
GO
CMD
finish
