#!/usr/bin/env bash
# Verification for T004 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "TestReviewQueries" <<'CMD'
gotest ./internal/synth/ '.*' TestReviewQueries
CMD
expect_ok "synth vet" <<'CMD'
go vet ./internal/synth/
CMD
expect_fail "negative: synth review imports no provider or gitops" <<'CMD'
test -f internal/synth/review.go || exit 125
grep -e 'internal/gitops' -e 'internal/provider' internal/synth/review.go
CMD
finish
