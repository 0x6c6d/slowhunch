#!/usr/bin/env bash
# Verification for T012 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "prompt tests" <<'CMD'
gotest "./internal/provider/..." 'TestPrompts' TestPromptsEmbedded TestPromptsOverride TestPromptsInvalid
CMD
expect_ok "three prompt files with role and version" <<'CMD'
for r in gate extract place; do
  f="internal/provider/prompts/$r.md"
  grep -qx "version: $r-1" "$f" && grep -qx "role: $r" "$f" && grep -q 'SLOWHUNCH-INTAKE' "$f" || { echo "bad $f"; exit 1; }
done
CMD
expect_fail "negative: prompt without version is rejected" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/provider"
)

func main() {
	if _, err := provider.ParsePrompt("gate", []byte("---\nrole: gate\nversion: gate-1\n---\nok\n")); err != nil {
		os.Exit(125)
	}
	if _, err := provider.ParsePrompt("gate", []byte("---\nrole: gate\n---\nno version\n")); err == nil {
		os.Exit(0)
	}
	if _, err := provider.ParsePrompt("gate", []byte("---\nrole: place\nversion: place-1\n---\nwrong role\n")); err == nil {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
