#!/usr/bin/env bash
# Verification for T024 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "verify tests" <<'CMD'
gotest "./internal/vault/... ./internal/cli/..." 'TestVerifyChecks|TestVerifyCorrupted' TestVerifyChecks TestVerifyCorrupted
CMD
expect_ok "binary: fresh vault verifies clean" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
"$t/sh" init "$t/v" >/dev/null || exit 1
"$t/sh" create topic --data '{"title":"Trees"}' --vault "$t/v" >/dev/null || exit 1
"$t/sh" verify --vault "$t/v" --json | grep -q '"problems": \[\]'
CMD
expect_fail "negative: wrong rev and dangling link exit 3, vault unchanged" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" create topic --data '{"title":"Trees"}' --vault "$t/v" >/dev/null || exit 125
sed -i 's/^rev: .*/rev: 000000/' "$t/v/topics/trees.md"
printf 'see [[ghost-id]]\n' >> "$t/v/topics/trees.md"
before="$(find "$t/v" -path "$t/v/.git" -prune -o -type f -print0 | sort -z | xargs -0 sha256sum)"
out="$("$t/sh" verify --vault "$t/v" --json)"; rc=$?
after="$(find "$t/v" -path "$t/v/.git" -prune -o -type f -print0 | sort -z | xargs -0 sha256sum)"
[ "$before" = "$after" ] || exit 0
[ "$rc" -eq 3 ] && grep -q 'REV_MISMATCH' <<<"$out" && grep -q 'DANGLING_LINK' <<<"$out" && grep -q '"id": "trees"' <<<"$out" && exit 1
exit 0
CMD
finish
