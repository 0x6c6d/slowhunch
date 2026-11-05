#!/usr/bin/env bash
# Verification for T025 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
# dod "<pkgs>" "<regex>" Pattern...: go test -v must exit 0, print no "--- FAIL" and at least one
# "--- PASS: <Pattern>..." line per named pattern (Global Definition of Done rule).
dod() {
  local pkgs="$1" run="$2" out p
  shift 2
  # shellcheck disable=SC2086
  out="$(go test $pkgs -count=1 -v -run "$run" 2>&1)" || { printf '%s\n' "$out" | grep -E -e '^(--- FAIL|FAIL|panic)' | tail -n 20; return 1; }
  if printf '%s\n' "$out" | grep -q -e '--- FAIL'; then printf '%s\n' "$out" | grep -e '--- FAIL' | head; return 1; fi
  for p in "$@"; do
    printf '%s\n' "$out" | grep -qE "^--- PASS: ${p}" || { echo "no PASS for ${p}"; return 1; }
  done
}
export -f dod
expect_ok "DoD 1: collide" <<'CMD'
dod ./... 'TestCollide' TestCollide
CMD
expect_ok "DoD 2: review" <<'CMD'
dod ./... 'TestReview' TestReview
CMD
expect_ok "DoD 3: wildcard" <<'CMD'
dod ./... 'TestWildcard' TestWildcard
CMD
expect_ok "DoD 4: keep" <<'CMD'
dod ./... 'TestKeep' TestKeep
CMD
expect_ok "DoD 5: batch" <<'CMD'
dod ./... 'TestBatch' TestBatch
CMD
expect_ok "DoD 6: pdf" <<'CMD'
dod ./... 'TestPDF' TestPDF
CMD
expect_ok "DoD 7: provider resolution, stats, caps" <<'CMD'
dod ./... 'TestResolveV2|TestStatsCollide|TestCaps' TestResolveV2 TestStatsCollide TestCaps
CMD
expect_ok "DoD 8: pull and push" <<'CMD'
dod ./... 'TestPull|TestPush' TestPull TestPush
CMD
expect_ok "DoD 9: sync, dry-run behind, search sync" <<'CMD'
dod ./... 'TestSync|TestDryRunBehind|TestSearchSync' TestSync TestDryRunBehind TestSearchSync
CMD
expect_ok "DoD 10: rebase conflict and pull failure" <<'CMD'
dod ./... 'TestRebaseConflict|TestPullFailed' TestRebaseConflict TestPullFailed
CMD
expect_ok "DoD 11: git policy" <<'CMD'
dod ./internal/policy/ 'TestNoForceGit|TestGitOnlyGitops' TestNoForceGit TestGitOnlyGitops
CMD
expect_ok "DoD 12: skill and README tests" <<'CMD'
dod ./internal/cli/ 'TestSkill|TestReadme' TestSkill TestReadme
CMD
VERIFY_TIMEOUT=900 expect_ok "DoD 12: README task verification" <<'CMD'
bash .harness/verify/v2/T023.sh
CMD
VERIFY_TIMEOUT=900 expect_ok "DoD 13: make check" <<'CMD'
make check
CMD
finish
