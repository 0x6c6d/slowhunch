#!/usr/bin/env bash
# Verification for T014 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "TestCapsTokensPerRun" <<'CMD'
out="$(go test ./internal/pipeline/ ./internal/cli/ -count=1 -v -run 'TestCapsTokensPerRun' 2>&1)" || { printf '%s\n' "$out" | tail -n 30; exit 1; }
printf '%s\n' "$out" | grep -qE '^--- PASS: TestCapsTokensPerRun ' || { echo "missing or not passing: TestCapsTokensPerRun"; exit 1; }
CMD
expect_ok "pipeline and cli packages" <<'CMD'
go test ./internal/pipeline/ ./internal/cli/ -count=1
CMD
expect_ok "tokens_per_run read in the pipeline" <<'CMD'
grep -rq --include=*.go -e 'TokensPerRun' internal/pipeline/
CMD
finish
