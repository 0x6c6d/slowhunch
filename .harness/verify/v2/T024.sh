#!/usr/bin/env bash
# Verification for T024 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "CLAUDE.md mentions the v2 architecture" <<'CMD'
grep -q -F -e 'internal/synth' CLAUDE.md && grep -q -F -e 'pdf.command' CLAUDE.md && grep -q -F -e 'RunArgv' CLAUDE.md
CMD
expect_fail "negative: Delegation workflow section differs from HEAD" <<'CMD'
a="$(git show HEAD:CLAUDE.md | sed -n '/^## Delegation workflow/,$p')"
b="$(sed -n '/^## Delegation workflow/,$p' CLAUDE.md)"
[ -n "$a" ] || exit 125
[ "$a" != "$b" ]
CMD
expect_ok "make check" <<'CMD'
make check
CMD
finish
