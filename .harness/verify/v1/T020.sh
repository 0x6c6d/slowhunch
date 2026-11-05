#!/usr/bin/env bash
# Verification for T020 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "TestSourceFacts" <<'CMD'
gotest "./internal/cli/..." 'TestSourceFacts' TestSourceFacts
CMD
expect_ok "source facts listed in help" <<'CMD'
go run ./cmd/slowhunch help | grep -q 'source facts'
CMD
expect_fail "negative: unknown source id is NOT_FOUND" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" source facts nope --vault "$t/v" --json | grep -q '"code": "NOT_FOUND"' && exit 1
exit 0
CMD
finish
