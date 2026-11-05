#!/usr/bin/env bash
# Verification for T040 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "agent sequence, schema and exit-code tests" <<'CMD'
gotest "./internal/cli/..." 'TestAgentSequence|TestSchemaAllCommands|TestExitCodesAll' TestAgentSequence TestSchemaAllCommands TestExitCodesAll
CMD
expect_ok "plan DoD CLI line" <<'CMD'
go test ./internal/cli/... -count=1 -run 'TestAgentSequence|TestHunchDue|TestSourceFacts|TestSchema|TestExitCodes'
CMD
expect_fail "negative: invalid config.json is a fatal error with exit 1" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
echo '{not json' > "$t/v/.slowhunch/config.json"
"$t/sh" list --vault "$t/v" --json >/dev/null 2>&1; rc=$?
[ "$rc" -eq 1 ] && exit 1
exit 0
CMD
finish
