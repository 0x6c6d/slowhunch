#!/usr/bin/env bash
# Verification for T003 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "new policy tests pass" <<'CMD'
gotest ./internal/policy/ '.*' TestNoForceGit TestGitOnlyGitops TestExecOnlyGitopsAndExecProvider
CMD
expect_fail "old push/pull word ban removed" <<'CMD'
grep -q -e 'func TestNoForbiddenGit' internal/policy/policy_test.go
CMD
expect_fail "negative: planted force argv in gitops fails TestNoForceGit" <<'CMD'
f=internal/gitops/zz_verify_neg.go
trap 'rm -f "$f"' EXIT
printf 'package gitops\n\nvar verifyNeg = []string{"push", "--force"}\n' > "$f"
go test ./internal/policy/ -count=1 -run '^TestNoForceGit$'
CMD
expect_fail "negative: planted +refspec literal fails TestNoForceGit" <<'CMD'
d=internal/zzverifyneg
trap 'rm -rf "$d"' EXIT
mkdir -p "$d"
printf 'package zzverifyneg\n\nvar Refspec = "+main"\n' > "$d/x.go"
go test ./internal/policy/ -count=1 -run '^TestNoForceGit$'
CMD
expect_fail "negative: planted git call outside gitops fails TestGitOnlyGitops" <<'CMD'
d=internal/zzverifyneggit
trap 'rm -rf "$d"' EXIT
mkdir -p "$d"
printf 'package zzverifyneggit\n\nimport "os/exec"\n\nfunc F() error { return exec.Command("git", "status").Run() }\n' > "$d/x.go"
go test ./internal/policy/ -count=1 -run '^TestGitOnlyGitops$'
CMD
expect_ok "planted files cleaned up and tree compiles" <<'CMD'
test ! -e internal/gitops/zz_verify_neg.go && test ! -e internal/zzverifyneg && test ! -e internal/zzverifyneggit && go vet ./internal/policy/
CMD
finish
