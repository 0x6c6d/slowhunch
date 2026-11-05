#!/usr/bin/env bash
# Verification for T002 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "cli core tests" <<'CMD'
gotest "./internal/cli/..." 'TestSchema|TestExitCodes|TestGlobalFlags|TestVaultLookup' TestSchema TestExitCodes TestGlobalFlags TestVaultLookup
CMD
expect_ok "errs package has passing tests" <<'CMD'
ls internal/errs/*_test.go >/dev/null && go test ./internal/errs/ -count=1
CMD
expect_ok "unknown command: exit 2, BAD_INPUT JSON with schema and hint" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/slowhunch" ./cmd/slowhunch || exit 1
out="$("$t/slowhunch" nosuchcmd --json)"; rc=$?
[ "$rc" -eq 2 ] && grep -q '"schema": 1' <<<"$out" && grep -q '"code": "BAD_INPUT"' <<<"$out" && grep -q '"hint"' <<<"$out"
CMD
expect_fail "negative: no arguments is a usage error (exit 2)" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/slowhunch" ./cmd/slowhunch || exit 125
"$t/slowhunch" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 2 ] && exit 1
[ "$rc" -eq 0 ] && exit 0
exit 125
CMD
expect_ok "help exits 0" <<'CMD'
go run ./cmd/slowhunch help >/dev/null
CMD
expect_ok "make check" <<'CMD'
make check
CMD
finish
