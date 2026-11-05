#!/usr/bin/env bash
# Verification for T020 (supervisor-owned; workers never edit it). Exit 0 = pass.
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
# remote_pair DIR: bare repo DIR/bare.git, vault DIR/a pushed to it with an upstream, clone DIR/b.
remote_pair() {
  mkdir -p "$1" &&
  git init -q --bare "$1/bare.git" &&
  "$SH" init "$1/a" >/dev/null &&
  git -C "$1/a" remote add origin "$1/bare.git" &&
  git -C "$1/a" push -q -u origin HEAD 2>/dev/null &&
  git clone -q "$1/bare.git" "$1/b" 2>/dev/null
}
# topic VAULT TITLE [global flags...]: create a topic, print the JSON result.
topic() { local v="$1" t="$2"; shift 2; "$SH" --vault "$v" --json "$@" create topic --data "{\"title\":\"$t\",\"body\":\"text of $t\"}"; }
export -f remote_pair topic
expect_ok "push tests" <<'CMD'
gotest ./internal/cli/ 'TestPush' TestPushOptIn TestPushDefaultOff TestPushFlags TestPushFailedRetried TestPushThenPull
CMD
expect_ok "cli package" <<'CMD'
go test ./internal/cli/ -count=1
CMD
expect_ok "git.push true updates the bare remote" <<'CMD'
remote_pair "$W/p" || exit 1
printf '%s\n' '{"git":{"push":true}}' > "$W/p/a/.slowhunch/config.json"
git -C "$W/p/a" commit -qam cfg || exit 1
out="$(topic "$W/p/a" "Pushed")" || { echo "$out"; exit 1; }
printf '%s' "$out" | grep -q -e '"pushed": true' || { echo "$out"; exit 1; }
[ "$(git -C "$W/p/bare.git" rev-parse HEAD)" = "$(git -C "$W/p/a" rev-parse HEAD)" ]
CMD
expect_fail "negative: default config pushes nothing" <<'CMD'
remote_pair "$W/d" || exit 125
before="$(git -C "$W/d/bare.git" rev-parse HEAD)"
topic "$W/d/a" "Local only" >/dev/null || exit 125
[ "$(git -C "$W/d/bare.git" rev-parse HEAD)" != "$before" ]
CMD
expect_fail "negative: --no-push beats git.push true" <<'CMD'
remote_pair "$W/n" || exit 125
printf '%s\n' '{"git":{"push":true}}' > "$W/n/a/.slowhunch/config.json"
git -C "$W/n/a" commit -qam cfg || exit 125
before="$(git -C "$W/n/bare.git" rev-parse HEAD)"
topic "$W/n/a" "Held back" --no-push >/dev/null || exit 125
[ "$(git -C "$W/n/bare.git" rev-parse HEAD)" != "$before" ]
CMD
expect_fail "negative: --push --no-push fails BAD_INPUT" <<'CMD'
remote_pair "$W/x" || exit 125
fails_with BAD_INPUT topic "$W/x/a" "Both" --push --no-push
CMD
finish
