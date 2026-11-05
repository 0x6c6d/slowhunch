#!/usr/bin/env bash
# Verification for T007 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "TestResolveV2" <<'CMD'
gotest ./internal/provider/ '.*' TestResolveV2
CMD
expect_ok "TestResolveV2Pipeline and pipeline package" <<'CMD'
gotest ./internal/pipeline/ '.*' TestResolveV2Pipeline
CMD
expect_ok "provider subpackages and cli" <<'CMD'
go test ./internal/provider/... ./internal/cli/ -count=1
CMD
expect_ok "ForRole takes a medium" <<'CMD'
grep -q -E -e 'func ForRole\(cfg vault\.Config, role, medium string\)' internal/provider/provider.go
CMD
finish
