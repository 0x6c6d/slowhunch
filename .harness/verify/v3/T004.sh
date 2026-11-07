#!/usr/bin/env bash
# Verification for T004 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "README exists with core headings" <<'CMD'
test -f README.md
grep -q -e '^## Installation' README.md
grep -q -e '^## Quick start' README.md
grep -q -e '^## Usage' README.md
grep -q -e '^## Development' README.md
CMD
expect_ok "README documents wikilink stripping" <<'CMD'
grep -n -i 'wikilink' README.md
grep -n -i -e 'not rewritten' -e 'are not changed' -e 'not modified' README.md
CMD
expect_fail "negative: no harness or placeholder text" <<'CMD'
grep -n -e '\.harness/' -e 'TODO' -e 'TBD' -e '<\.\.\.>' README.md
CMD
expect_ok "existing README test" <<'CMD'
timeout 300 go test ./internal/cli/ -count=1 -run README
CMD
expect_ok "make check" <<'CMD'
make check
CMD
finish
