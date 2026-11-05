#!/usr/bin/env bash
# Verification for T026 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "stats tests" <<'CMD'
gotest "./internal/report/... ./internal/cli/..." 'TestStatsCompute|TestStatsCommand' TestStatsCompute TestStatsCommand
CMD
expect_ok "binary: stats on a fresh vault" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
"$t/sh" init "$t/v" >/dev/null || exit 1
out="$("$t/sh" stats --vault "$t/v" --json)" || exit 1
grep -q '"schema": 1' <<<"$out" && grep -q '"asserts_reads_ratio": null' <<<"$out" && grep -q '"atoms_by_medium": {}' <<<"$out" && grep -q '"tokens_by_role"' <<<"$out"
CMD
expect_fail "negative: stats outside a vault is NOT_FOUND" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
cd "$t" && env -u SLOWHUNCH_VAULT "$t/sh" stats --json | grep -q '"code": "NOT_FOUND"' && exit 1
exit 0
CMD
finish
