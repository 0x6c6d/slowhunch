#!/usr/bin/env bash
# Verification for T022 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "ranking unit tests" <<'CMD'
gotest "./internal/search/..." 'TestSearchScore|TestSearchBudget' TestSearchScore TestSearchBudget
CMD
expect_ok "seeded vault search test" <<'CMD'
gotest "./internal/cli/..." 'TestSearchSeeded' TestSearchSeeded
CMD
expect_ok "binary: search on an empty vault returns zero hits" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
"$t/sh" init "$t/v" >/dev/null || exit 1
out="$("$t/sh" search "trees leaf" --vault "$t/v" --json)" || exit 1
grep -q '"schema": 1' <<<"$out" && grep -q '"count": 0' <<<"$out"
CMD
expect_fail "negative: invalid stance and unknown hunch are usage errors" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" search "x" --vault "$t/v" >/dev/null 2>&1 || exit 125
"$t/sh" search "x" --stance shouts --vault "$t/v" >/dev/null 2>&1; a=$?
"$t/sh" search "x" --hunch ghost --vault "$t/v" >/dev/null 2>&1; b=$?
[ "$a" -eq 2 ] && [ "$b" -eq 2 ] && exit 1
exit 0
CMD
finish
