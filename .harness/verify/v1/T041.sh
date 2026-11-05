#!/usr/bin/env bash
# Verification for T041 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "perf test passes (verify and search < 5 s on 2,000 atoms)" <<'CMD'
gotest "./internal/cli/..." 'TestPerf' TestPerfVerifyAndSearch
CMD
expect_ok "plan DoD perf line" <<'CMD'
go test ./... -run 'TestPerf' -count=1
CMD
expect_fail "negative: the perf test is never skipped" <<'CMD'
out="$(go test ./internal/cli/... -run 'TestPerf' -count=1 -v -short 2>&1)" || exit 125
grep -q '^--- PASS: TestPerfVerifyAndSearch' <<<"$out" || exit 125
grep -q -- '--- SKIP' <<<"$out"
CMD
finish
