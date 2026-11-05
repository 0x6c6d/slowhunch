#!/usr/bin/env bash
# Verification for T002 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "TestErrsV2 and errs package" <<'CMD'
gotest ./internal/errs/ '.*' TestErrsV2
CMD
expect_ok "codes present" <<'CMD'
f=internal/errs/errs.go
grep -q -e '"CAP_REACHED"' $f && grep -q -e '"PULL_FAILED"' $f && grep -q -e '"PUSH_FAILED"' $f && grep -q -e '"BEHIND"' $f
CMD
finish
