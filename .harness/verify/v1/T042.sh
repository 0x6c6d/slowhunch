#!/usr/bin/env bash
# Verification for T042 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "policy tests" <<'CMD'
gotest "./internal/policy/..." 'TestNoForbiddenGit|TestNetworkOnlyOpenAI|TestExecOnlyGitopsAndExecProvider|TestStdlibOnly|TestTestGuard' TestNoForbiddenGit TestNetworkOnlyOpenAI TestExecOnlyGitopsAndExecProvider TestStdlibOnly TestTestGuard
CMD
expect_ok "plan DoD grep line" <<'CMD'
test -z "$(git grep -nE 'git (push|pull)|--force' -- '*.go' ':!*_test.go')"
CMD
expect_fail "negative: a planted forbidden git verb makes TestNoForbiddenGit fail" <<'CMD'
test -f internal/policy/policy_test.go || exit 125
d="$(mktemp -d internal/zzplantXXXXXX)" || exit 125
trap 'rm -r -- "$d"' EXIT
printf 'package zz\n\nconst c = "git push origin main"\n' > "$d/plant.go"
go test ./internal/policy/ -count=1 -run 'TestNoForbiddenGit' >/dev/null 2>&1
CMD
finish
