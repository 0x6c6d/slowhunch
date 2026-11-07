#!/usr/bin/env bash
# Verification for T000 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "working tree clean" <<'CMD'
test -z "$(git status --porcelain)"
CMD
expect_ok "make check on the v2 code" <<'CMD'
make check
CMD
expect_ok "go available" <<'CMD'
go version
CMD
finish
