#!/usr/bin/env bash
# Verification for T015 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "TestCollideSelect and synth package" <<'CMD'
gotest ./internal/synth/ '.*' TestCollideSelect
CMD
expect_ok "Select and Pool exported" <<'CMD'
grep -q -e '^func Select(' internal/synth/collide.go && grep -q -e '^func Pool(' internal/synth/collide.go && grep -q -e '^func ExistingPairs(' internal/synth/collide.go
CMD
expect_fail "negative: selection imports no provider or gitops" <<'CMD'
test -f internal/synth/collide.go || exit 125
grep -e 'internal/gitops' -e 'internal/provider' internal/synth/collide.go
CMD
finish
