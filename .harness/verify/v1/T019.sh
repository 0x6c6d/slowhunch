#!/usr/bin/env bash
# Verification for T019 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "hunch tests" <<'CMD'
gotest "./internal/cli/..." 'TestHunch' TestHunchAdd TestHunchEvidence TestHunchState TestHunchDue
CMD
expect_ok "binary: hunch due lists past revisit only" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
"$t/sh" init "$t/v" >/dev/null || exit 1
"$t/sh" hunch add "Old question here" --revisit 2000-01-01 --vault "$t/v" >/dev/null || exit 1
"$t/sh" hunch add "Future question here" --revisit 2999-01-01 --vault "$t/v" >/dev/null || exit 1
out="$("$t/sh" hunch due --vault "$t/v" --json)" || exit 1
grep -q '"id": "old-question-here"' <<<"$out" && ! grep -q 'future-question-here' <<<"$out"
CMD
expect_fail "negative: invalid relation is refused and the hunch stays unchanged" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
"$t/sh" hunch add "Why do lines stick" --vault "$t/v" >/dev/null || exit 125
out="$("$t/sh" create note --data '{"claim":"c","evidence":"e","stance":"asserts"}' --vault "$t/v" --json)" || exit 125
nid="$(sed -n 's/.*"id": "\([a-z0-9-]*\)".*/\1/p' <<<"$out" | head -n 1)"
cp "$t/v/hunches/why-do-lines-stick.md" "$t/before"
"$t/sh" hunch evidence why-do-lines-stick --note "$nid" --relation proves --vault "$t/v" >/dev/null 2>&1; rc=$?
cmp -s "$t/before" "$t/v/hunches/why-do-lines-stick.md" || exit 0
[ "$rc" -eq 2 ] && exit 1
exit 0
CMD
finish
