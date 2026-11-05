#!/usr/bin/env bash
# Verification for T018 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "link and graph tests" <<'CMD'
gotest "./internal/cli/..." 'TestCRUDLink|TestCRUDUnlink|TestCRUDBacklinks|TestCRUDGraph' TestCRUDLink TestCRUDLinkIdempotent TestCRUDUnlink TestCRUDBacklinks TestCRUDGraphDepth TestCRUDLinkMissing
CMD
expect_ok "binary: link then backlinks, target untouched" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
"$t/sh" init "$t/v" >/dev/null || exit 1
"$t/sh" create topic --data '{"title":"Trees"}' --vault "$t/v" >/dev/null || exit 1
"$t/sh" create topic --data '{"title":"Leaves"}' --vault "$t/v" >/dev/null || exit 1
cp "$t/v/topics/trees.md" "$t/before"
"$t/sh" link leaves trees --vault "$t/v" --json | grep -q '"schema": 1' || exit 1
cmp -s "$t/before" "$t/v/topics/trees.md" || exit 1
"$t/sh" backlinks trees --vault "$t/v" --json | tr -d ' \n' | grep -q '"backlinks":\["leaves"\]'
CMD
expect_fail "negative: link to a missing id fails and writes nothing" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" create topic --data '{"title":"Leaves"}' --vault "$t/v" >/dev/null || exit 125
cp "$t/v/topics/leaves.md" "$t/before"
out="$("$t/sh" link leaves ghost --vault "$t/v" --json)"; rc=$?
cmp -s "$t/before" "$t/v/topics/leaves.md" || exit 0
[ "$rc" -eq 2 ] && grep -q '"code": "NOT_FOUND"' <<<"$out" && exit 1
exit 0
CMD
finish
