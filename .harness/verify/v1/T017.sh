#!/usr/bin/env bash
# Verification for T017 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "list tests" <<'CMD'
gotest "./internal/cli/..." 'TestCRUDList' TestCRUDList TestCRUDListFilters TestCRUDListBadFilter
CMD
expect_ok "binary: list --type filters and counts" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
"$t/sh" init "$t/v" >/dev/null || exit 1
"$t/sh" create topic --data '{"title":"Trees"}' --vault "$t/v" >/dev/null || exit 1
"$t/sh" create hunch --data '{"question":"Why do lines stick?"}' --vault "$t/v" >/dev/null || exit 1
out="$("$t/sh" list --type hunch --vault "$t/v" --json)" || exit 1
grep -q '"count": 1' <<<"$out" && grep -q '"id": "why-do-lines-stick"' <<<"$out" && ! grep -q '"id": "trees"' <<<"$out"
CMD
expect_fail "negative: unknown stance filter is a usage error" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" list --stance shouts --vault "$t/v" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 2 ] && exit 1
exit 0
CMD
finish
