#!/usr/bin/env bash
# Verification for T000 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "go 1.27 installed" <<'CMD'
go version | grep -q 'go1.27'
CMD
expect_ok "git installed" <<'CMD'
git --version
CMD
expect_ok "make installed" <<'CMD'
make --version >/dev/null
CMD
expect_ok "gofmt installed" <<'CMD'
gofmt -h 2>&1 | grep -q usage
CMD
expect_ok "working tree clean" <<'CMD'
test -z "$(git status --porcelain)"
CMD
expect_ok "repo top level is the project root" <<'CMD'
test "$(git rev-parse --show-toplevel)" = /home/sa/workspace/code/slowhunch
CMD
expect_ok "no go.sum (stdlib only)" <<'CMD'
test ! -e go.sum
CMD
finish
