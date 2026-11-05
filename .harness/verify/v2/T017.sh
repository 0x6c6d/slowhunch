#!/usr/bin/env bash
# Verification for T017 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
# CLI fixture: a throw-away work dir, an isolated git identity and a fresh binary in $SH.
W="$(mktemp -d)"; export W
trap 'rm -rf "$W"' EXIT
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0
export GIT_AUTHOR_NAME=verify GIT_AUTHOR_EMAIL=verify@example.invalid
export GIT_COMMITTER_NAME=verify GIT_COMMITTER_EMAIL=verify@example.invalid
unset SLOWHUNCH_VAULT
SH="$W/slowhunch"; export SH
# fails_with CODE cmd...: returns cmd's exit status when it failed AND its output names CODE; else 125.
fails_with() {
  local code="$1" out rc
  shift
  out="$("$@" 2>&1)"; rc=$?
  if [ "$rc" -eq 0 ] || ! printf '%s' "$out" | grep -q -e "\"code\": \"$code\""; then
    printf '%s\n' "$out" | tail -n 8
    return 125
  fi
  return "$rc"
}
# treehash DIR: one hash over every file under DIR except .git/.
treehash() { (cd "$1" && find . -path ./.git -prune -o -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum); }
export -f fails_with treehash
expect_ok "build binary" <<'CMD'
go build -o "$SH" ./cmd/slowhunch
CMD
expect_ok "keep tests" <<'CMD'
gotest ./internal/cli/ 'TestKeep' TestKeepNamed TestKeepNearest TestKeepFailures
CMD
expect_ok "keep appends a dated block and commits" <<'CMD'
"$SH" init "$W/v" >/dev/null || exit 1
"$SH" --vault "$W/v" --json create topic --data '{"title":"Rivers","body":"water flows"}' >/dev/null || exit 1
head="$(git -C "$W/v" rev-parse HEAD)"
"$SH" --vault "$W/v" --json keep "rivers carve valleys" --ids rivers --topic rivers >/dev/null || exit 1
grep -q -E -e '^### [0-9]{4}-[0-9]{2}-[0-9]{2}$' "$W/v/topics/rivers.md" && grep -q -e 'rivers carve valleys' "$W/v/topics/rivers.md" && grep -q -e '\[\[rivers\]\]' "$W/v/topics/rivers.md" || exit 1
[ "$head" != "$(git -C "$W/v" rev-parse HEAD)" ] && test -z "$(git -C "$W/v" status --porcelain)"
CMD
expect_fail "negative: unknown id fails NOT_FOUND, HEAD unchanged" <<'CMD'
"$SH" init "$W/a" >/dev/null || exit 125
"$SH" --vault "$W/a" --json create topic --data '{"title":"Rivers","body":"water"}' >/dev/null || exit 125
head="$(git -C "$W/a" rev-parse HEAD)"
fails_with NOT_FOUND "$SH" --vault "$W/a" --json keep "text" --ids no-such-id; rc=$?
[ "$head" = "$(git -C "$W/a" rev-parse HEAD)" ] || exit 125
exit "$rc"
CMD
expect_fail "negative: 2,001 characters fail TOO_LONG" <<'CMD'
"$SH" init "$W/b" >/dev/null || exit 125
"$SH" --vault "$W/b" --json create topic --data '{"title":"Rivers","body":"water"}' >/dev/null || exit 125
long="$(head -c 2001 /dev/zero | tr '\0' 'a')"
before="$(treehash "$W/b")"
fails_with TOO_LONG "$SH" --vault "$W/b" --json keep "$long" --ids rivers; rc=$?
[ "$before" = "$(treehash "$W/b")" ] || exit 125
exit "$rc"
CMD
expect_fail "negative: no topic in the vault fails NOT_FOUND" <<'CMD'
"$SH" init "$W/c" >/dev/null || exit 125
"$SH" --vault "$W/c" --json hunch add "do rivers remember" >/dev/null || exit 125
id="$(ls "$W/c/hunches" | head -n 1)"; id="${id%.md}"
fails_with NOT_FOUND "$SH" --vault "$W/c" --json keep "text" --ids "$id"
CMD
finish
