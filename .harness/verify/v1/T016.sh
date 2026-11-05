#!/usr/bin/env bash
# Verification for T016 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "delete tests" <<'CMD'
gotest "./internal/cli/..." 'TestCRUDDelete' TestCRUDDeleteSoft TestCRUDDeleteHardNeedsYes TestCRUDDeleteHard TestCRUDDeleteMissing TestCRUDDeleteDryRun
CMD
expect_ok "binary: soft delete moves to archive and commits" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
"$t/sh" init "$t/v" >/dev/null || exit 1
"$t/sh" create topic --data '{"title":"Trees"}' --vault "$t/v" >/dev/null || exit 1
"$t/sh" delete trees --vault "$t/v" --json | grep -q '"archived": "archive/trees.md"' || exit 1
test -f "$t/v/archive/trees.md" && test ! -e "$t/v/topics/trees.md" || exit 1
test -z "$(git -C "$t/v" status --porcelain -- topics archive)"
CMD
expect_fail "negative: hard delete without --yes is refused and keeps the file" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" create topic --data '{"title":"Trees"}' --vault "$t/v" >/dev/null || exit 125
"$t/sh" delete trees --hard --vault "$t/v" >/dev/null 2>&1; rc=$?
test -f "$t/v/topics/trees.md" || exit 0
[ "$rc" -eq 2 ] && exit 1
exit 0
CMD
expect_fail "negative: deleting an unknown id fails" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" delete no-such-id --vault "$t/v" --json | grep -q '"code": "NOT_FOUND"' && exit 1
exit 0
CMD
finish
