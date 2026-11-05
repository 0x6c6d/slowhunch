#!/usr/bin/env bash
# Verification for T044 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "CLAUDE.md has Commands and Architecture, placeholder gone" <<'CMD'
grep -qx '## Commands' CLAUDE.md && grep -qx '## Architecture' CLAUDE.md && grep -q 'make check' CLAUDE.md && grep -q 'internal/gitops' CLAUDE.md && ! grep -q 'Fill in once code exists' CLAUDE.md
CMD
expect_ok "new sections come before the Delegation workflow section" <<'CMD'
a=$(grep -n '^## Architecture$' CLAUDE.md | cut -d: -f1); d=$(grep -n "^## Delegation workflow" CLAUDE.md | cut -d: -f1)
[ -n "$a" ] && [ -n "$d" ] && [ "$a" -lt "$d" ]
CMD
expect_fail "negative: the Delegation workflow section differs from the committed version" <<'CMD'
git show 14d7ca6:CLAUDE.md > /dev/null 2>&1 || exit 125
old="$(git show 14d7ca6:CLAUDE.md | sed -n '/^## Delegation workflow/,$p')"
new="$(sed -n '/^## Delegation workflow/,$p' CLAUDE.md)"
[ -n "$old" ] || exit 125
[ "$old" != "$new" ]
CMD
expect_ok "no docs/ folder" <<'CMD'
test ! -e docs
CMD
finish
