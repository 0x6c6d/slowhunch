#!/usr/bin/env bash
# Verification for T009 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "TestRunArgv and exec provider tests" <<'CMD'
gotest ./internal/provider/exec/ '.*' TestRunArgv
CMD
expect_ok "policy tests" <<'CMD'
go test ./internal/policy/ -count=1
CMD
expect_ok "os/exec only in gitops and provider/exec" <<'CMD'
bad="$(grep -rl --include=*.go -e '"os/exec"' internal cmd | grep -v -e '_test\.go$' | grep -v -e '^internal/gitops/' -e '^internal/provider/exec/')"
[ -z "$bad" ] || { echo "$bad"; exit 1; }
CMD
expect_ok "RunArgv and IsNotFound exported" <<'CMD'
grep -q -e '^func RunArgv(' internal/provider/exec/exec.go && grep -q -e '^func IsNotFound(' internal/provider/exec/exec.go
CMD
expect_fail "negative: no shell literal in the exec provider" <<'CMD'
grep -n -e '"sh"' -e '"bash"' -e '"-c"' -e '"/bin/sh"' internal/provider/exec/exec.go
CMD
finish
