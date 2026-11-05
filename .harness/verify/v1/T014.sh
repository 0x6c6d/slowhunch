#!/usr/bin/env bash
# Verification for T014 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "create/get tests" <<'CMD'
gotest "./internal/cli/..." 'TestCRUDCreate|TestCRUDGet' TestCRUDCreateGet TestCRUDCreateCollision TestCRUDCreateInvalid TestCRUDGetMissing
CMD
expect_ok "binary: create hunch, get it back, committed" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
"$t/sh" init "$t/v" >/dev/null || exit 1
out="$("$t/sh" create hunch --data '{"question":"Why do some lines stick?"}' --vault "$t/v" --json)" || exit 1
grep -q '"id": "why-do-some-lines-stick"' <<<"$out" || exit 1
g="$("$t/sh" get why-do-some-lines-stick --vault "$t/v" --json)" || exit 1
grep -q '"schema": 1' <<<"$g" && grep -qE '"rev": "[0-9a-f]{6}"' <<<"$g" || exit 1
git -C "$t/v" log -1 --format=%s | grep -q '^slowhunch: create why-do-some-lines-stick'
CMD
expect_fail "negative: confidence on a crafts note is refused, nothing written" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" create note --data '{"claim":"c","evidence":"e","stance":"crafts","locator":"line 1","confidence":"0.5"}' --vault "$t/v" --json >/dev/null; rc=$?
ls "$t/v/notes" | grep -q . && exit 0
[ "$rc" -eq 2 ] && exit 1
exit 0
CMD
finish
