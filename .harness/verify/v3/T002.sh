#!/usr/bin/env bash
# Verification for T002 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "e2e wikilink tests" <<'CMD'
gotest ./internal/cli/ 'Wikilink' TestAddWikilinkVerifyClean TestAddRawWikilinkEvidenceRejected
CMD
expect_ok "test file covers verify output and raw form" <<'CMD'
grep -q 'DANGLING_LINK' internal/cli/wikilink_e2e_test.go && grep -q '\[\[bloodhound\]\]' internal/cli/wikilink_e2e_test.go
CMD
expect_fail "negative: tests must not skip" <<'CMD'
grep -n 't.Skip' internal/cli/wikilink_e2e_test.go
CMD
expect_ok "make check" <<'CMD'
make check
CMD
finish
