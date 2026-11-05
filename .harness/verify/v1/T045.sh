#!/usr/bin/env bash
# Verification for T045 (supervisor-owned; workers never edit it). Exit 0 = pass.
# Global Definition of Done of .harness/versions/v1/plan.md, line by line.
. "$(dirname "$0")/../_lib.sh"
expect_ok "DoD: init tests" <<'CMD'
go test ./internal/cli/... -count=1 -run TestInit
CMD
expect_ok "DoD: frontmatter and vault tests" <<'CMD'
go test ./internal/frontmatter/... ./internal/vault/... -count=1
CMD
expect_ok "DoD: pipeline TestAdd suite" <<'CMD'
gotest "./internal/pipeline/..." 'TestAdd' TestAddLarkin TestAddEvidenceDrops TestAddInputErrors TestAddIdempotent TestAddDryRun TestAddGateError TestAddForce TestAddPlaceThresholds TestAddContradiction
CMD
expect_ok "DoD: CLI agent sequence, hunch due, source facts, schema, exit codes" <<'CMD'
gotest "./internal/cli/..." 'TestAgentSequence|TestHunchDue|TestSourceFacts|TestSchema|TestExitCodes' TestAgentSequence TestHunchDue TestSourceFacts TestSchema TestExitCodes TestExitCodesAll
CMD
expect_ok "DoD: search, lint, verify, stats, skill" <<'CMD'
go test ./internal/search/... ./internal/cli/... -count=1 -run 'TestSearch|TestLint|TestVerify|TestStats|TestSkill'
CMD
expect_ok "DoD: provider and intake tests" <<'CMD'
go test ./internal/provider/... ./internal/intake/... -count=1
CMD
expect_ok "DoD: conflict and exact commit" <<'CMD'
go test ./internal/gitops/... ./internal/pipeline/... -count=1 -run 'TestConflict|TestCommitExact'
CMD
expect_ok "DoD: no push, pull or force in non-test Go code" <<'CMD'
test -z "$(git grep -nE 'git (push|pull)|--force' -- '*.go' ':!*_test.go'; grep -rnE --include='*.go' 'git (push|pull)|--force' internal cmd | grep -v '_test\.go:')"
CMD
expect_fail "negative: forbidden git verbs are absent from untracked non-test Go files too" <<'CMD'
grep -rlE --include='*.go' 'git (push|pull)|--force' internal cmd | grep -v '_test\.go$'
CMD
expect_ok "DoD: perf (verify and search < 5 s on 2,000 atoms)" <<'CMD'
go test ./... -run 'TestPerf' -count=1
CMD
expect_ok "DoD: README verification script and quick start test" <<'CMD'
bash .harness/verify/v1/T043.sh && go test ./... -count=1 -run TestReadmeQuickstart
CMD
expect_ok "DoD: make check" <<'CMD'
make check
CMD
finish
