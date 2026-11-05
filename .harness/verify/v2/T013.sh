#!/usr/bin/env bash
# Verification for T013 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "TestCapsRunsPerDay and pipeline package" <<'CMD'
gotest ./internal/pipeline/ '.*' TestCapsRunsPerDay
CMD
expect_ok "vault and cli packages" <<'CMD'
go test ./internal/vault/ ./internal/cli/ -count=1
CMD
expect_ok "state key and code used" <<'CMD'
grep -rq -e '"runs_by_day' internal/vault/ && grep -rq --include=*.go -e 'CapReached' internal/pipeline/
CMD
finish
