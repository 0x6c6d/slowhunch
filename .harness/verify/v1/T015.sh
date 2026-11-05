#!/usr/bin/env bash
# Verification for T015 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "update tests" <<'CMD'
gotest "./internal/cli/..." 'TestCRUDUpdate' TestCRUDUpdatePatch TestCRUDUpdateStaleRev TestCRUDUpdateNoRev TestCRUDUpdateEvidence TestCRUDUpdateAppend TestCRUDUpdateDelete
CMD
expect_ok "binary: patch with current rev succeeds and commits" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
"$t/sh" init "$t/v" >/dev/null || exit 1
rev="$("$t/sh" create topic --data '{"title":"Trees"}' --vault "$t/v" --json | sed -n 's/.*"rev": "\([0-9a-f]\{6\}\)".*/\1/p' | head -n 1)"
[ -n "$rev" ] || exit 1
"$t/sh" update trees --rev "$rev" --data '{"tags":["poetry"]}' --vault "$t/v" --json | grep -q '"schema": 1' || exit 1
grep -q 'poetry' "$t/v/topics/trees.md" && git -C "$t/v" log -1 --format=%s | grep -q '^slowhunch: update trees'
CMD
expect_fail "negative: stale rev exits 4 with CONFLICT_REV and current content, file unchanged" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" create topic --data '{"title":"Trees"}' --vault "$t/v" >/dev/null || exit 125
cp "$t/v/topics/trees.md" "$t/before"
out="$("$t/sh" update trees --rev 000000 --data '{"tags":["x"]}' --vault "$t/v" --json)"; rc=$?
cmp -s "$t/before" "$t/v/topics/trees.md" || exit 0
[ "$rc" -eq 4 ] && grep -q '"code": "CONFLICT_REV"' <<<"$out" && grep -q '"current":' <<<"$out" && exit 1
exit 0
CMD
expect_fail "negative: update without --rev is a usage error" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" create topic --data '{"title":"Trees"}' --vault "$t/v" >/dev/null || exit 125
"$t/sh" update trees --data '{"tags":["x"]}' --vault "$t/v" >/dev/null 2>&1; rc=$?
grep -q '^  - x' "$t/v/topics/trees.md" && exit 0
[ "$rc" -eq 2 ] && exit 1
exit 0
CMD
expect_fail "negative: evidence is never rewritten" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
out="$("$t/sh" create note --data '{"claim":"c","evidence":"original words","stance":"asserts"}' --vault "$t/v" --json)" || exit 125
id="$(sed -n 's/.*"id": "\([a-z0-9-]*\)".*/\1/p' <<<"$out" | head -n 1)"; rev="$(sed -n 's/.*"rev": "\([0-9a-f]\{6\}\)".*/\1/p' <<<"$out" | head -n 1)"
"$t/sh" update "$id" --rev "$rev" --data '{"evidence":"rewritten"}' --vault "$t/v" >/dev/null 2>&1; rc=$?
grep -q 'rewritten' "$t/v/notes/$id.md" && exit 0
[ "$rc" -eq 2 ] && exit 1
exit 0
CMD
finish
