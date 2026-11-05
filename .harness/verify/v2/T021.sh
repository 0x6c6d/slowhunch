#!/usr/bin/env bash
# Verification for T021 (supervisor-owned; workers never edit it). Exit 0 = pass.
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
expect_ok "sync, dry-run and search tests" <<'CMD'
gotest ./internal/cli/ 'TestSync|TestDryRunBehind|TestSearchSync' TestSyncPullPush TestSyncNoRemote TestSyncFlags TestDryRunBehind TestSearchSync
CMD
expect_ok "cli package" <<'CMD'
go test ./internal/cli/ -count=1
CMD
expect_ok "sync pulls then pushes" <<'CMD'
remote_pair "$W/s" || exit 1
topic "$W/s/a" "From A" >/dev/null && git -C "$W/s/a" push -q 2>/dev/null || exit 1
topic "$W/s/b" "From B" --no-pull >/dev/null || exit 1
out="$("$SH" --vault "$W/s/b" --json sync)" || { echo "$out"; exit 1; }
printf '%s' "$out" | grep -q -e '"pulled": 1' && printf '%s' "$out" | grep -q -e '"pushed": true' || { echo "$out"; exit 1; }
[ "$(git -C "$W/s/bare.git" rev-parse HEAD)" = "$(git -C "$W/s/b" rev-parse HEAD)" ] && test -z "$(git -C "$W/s/b" status --porcelain)"
CMD
expect_ok "dry-run reports BEHIND and leaves the tree byte-identical" <<'CMD'
remote_pair "$W/d" || exit 1
topic "$W/d/a" "Ahead" >/dev/null && git -C "$W/d/a" push -q 2>/dev/null || exit 1
git -C "$W/d/b" fetch -q 2>/dev/null || exit 1
head="$(git -C "$W/d/b" rev-parse HEAD)"; before="$(treehash "$W/d/b")"
out="$(topic "$W/d/b" "Dry" --dry-run)" || { echo "$out"; exit 1; }
printf '%s' "$out" | grep -q -e 'BEHIND' && printf '%s' "$out" | grep -q -e '"behind": 1' || { echo "$out"; exit 1; }
[ "$head" = "$(git -C "$W/d/b" rev-parse HEAD)" ] && [ "$before" = "$(treehash "$W/d/b")" ]
CMD
expect_ok "search does not pull, search --sync does" <<'CMD'
remote_pair "$W/q" || exit 1
topic "$W/q/a" "Remote one" >/dev/null && git -C "$W/q/a" push -q 2>/dev/null || exit 1
head="$(git -C "$W/q/b" rev-parse HEAD)"
"$SH" --vault "$W/q/b" --json search "remote" >/dev/null || exit 1
[ "$head" = "$(git -C "$W/q/b" rev-parse HEAD)" ] || exit 1
"$SH" --vault "$W/q/b" --json --sync search "remote" >/dev/null || exit 1
[ "$head" != "$(git -C "$W/q/b" rev-parse HEAD)" ]
CMD
expect_fail "negative: sync without a remote fails NOT_FOUND" <<'CMD'
"$SH" init "$W/v" >/dev/null || exit 125
fails_with NOT_FOUND "$SH" --vault "$W/v" --json sync
CMD
finish
