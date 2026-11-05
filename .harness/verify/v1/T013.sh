#!/usr/bin/env bash
# Verification for T013 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "init tests" <<'CMD'
gotest "./internal/cli/..." 'TestInit' TestInitCreates TestInitTwice TestInitKeepsForeignAgentsFile TestInitGitMissing TestInitProtected
CMD
expect_ok "binary init: files, identical agent files, one commit, prompts" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
out="$("$t/sh" init "$t/v" --json)" || exit 1
grep -q '"schema": 1' <<<"$out" || exit 1
cmp -s "$t/v/AGENTS.md" "$t/v/CLAUDE.md" || exit 1
grep -q 'slowhunch verify' "$t/v/AGENTS.md" && grep -q 'slowhunch skill' "$t/v/AGENTS.md" || exit 1
for r in gate extract place; do test -s "$t/v/.slowhunch/prompts/$r.md" || exit 1; done
grep -qxF '.slowhunch/cache/' "$t/v/.gitignore" || exit 1
[ "$(git -C "$t/v" rev-list --count HEAD)" = 1 ] || exit 1
git -C "$t/v" show --name-only --format= HEAD | grep -qx 'AGENTS.md'
CMD
expect_fail "negative: second init never overwrites a customised CLAUDE.md" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
echo "owner text" >> "$t/v/CLAUDE.md"
cp "$t/v/CLAUDE.md" "$t/before"
out="$("$t/sh" init "$t/v" --json)" || exit 125
grep -q 'EXISTS' <<<"$out" || exit 0
cmp -s "$t/before" "$t/v/CLAUDE.md" || exit 0
exit 1
CMD
expect_fail "negative: init refuses a protected path under the test guard" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
SLOWHUNCH_TEST=1 SLOWHUNCH_PROTECTED="$t" "$t/sh" init "$t/v" >/dev/null 2>&1; rc=$?
test -e "$t/v/.slowhunch" && exit 0
[ "$rc" -eq 2 ] && exit 1
exit 0
CMD
finish
