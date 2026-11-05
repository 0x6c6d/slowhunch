#!/usr/bin/env bash
# Verification for T029 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "exec provider tests" <<'CMD'
gotest "./internal/provider/exec/..." 'TestExec' TestExecRoundTrip TestExecScrubbedEnv TestExecNoShell TestExecBadOutput TestExecFailExit TestExecTimeout TestExecEmptyCommand
CMD
expect_ok "os/exec imported only by gitops and provider/exec (non-test files)" <<'CMD'
test -z "$(grep -rl --include='*.go' '"os/exec"' internal cmd | grep -v '_test\.go$' | grep -v -e '^internal/gitops/' -e '^internal/provider/exec/')"
CMD
expect_fail "negative: empty command is refused" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	execprov "github.com/0x6c6d/slowhunch/internal/provider/exec"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	if _, err := execprov.New(vault.ProviderConfig{Name: "exec", Command: []string{"true"}}); err != nil {
		os.Exit(125)
	}
	for _, c := range [][]string{nil, {}, {""}} {
		if _, err := execprov.New(vault.ProviderConfig{Name: "exec", Command: c}); err == nil {
			os.Exit(0)
		}
	}
	os.Exit(1)
}
GO
CMD
expect_fail "negative: no shell invocation in the exec provider source" <<'CMD'
ls internal/provider/exec/exec.go >/dev/null 2>&1 || exit 125
grep -nE -e '"(/bin/)?(ba)?sh"' -e '"-c"' -e 'os\.Environ\(\)' internal/provider/exec/exec.go
CMD
finish
