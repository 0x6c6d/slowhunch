#!/usr/bin/env bash
# Verification for T005 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "wikilink in the intake bullet" <<'CMD'
grep -n 'internal/intake' CLAUDE.md | grep -i wikilink
CMD
expect_ok "Delegation workflow section unchanged" <<'CMD'
sec() { sed -n '/^## Delegation workflow/,$p'; }
test "$(git show HEAD:CLAUDE.md | sec)" = "$(sec < CLAUDE.md)"
CMD
expect_ok "make check" <<'CMD'
make check
CMD
finish
