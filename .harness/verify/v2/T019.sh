#!/usr/bin/env bash
# Verification for T019 (supervisor-owned; workers never edit it). Exit 0 = pass.
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
expect_ok "pull tests" <<'CMD'
gotest ./internal/cli/ 'TestPull|TestRebaseConflict' TestPullBeforeWrite TestPullNoPullFlag TestPullNoRemote TestPullFailed TestPullFailedRequire TestRebaseConflictCLI
CMD
expect_ok "cli and pipeline packages" <<'CMD'
go test ./internal/cli/ ./internal/pipeline/ -count=1
CMD
expect_ok "B pulls A's pushed commit before writing" <<'CMD'
remote_pair "$W/p" || exit 1
topic "$W/p/a" "From A" >/dev/null || exit 1
git -C "$W/p/a" push -q 2>/dev/null || exit 1
out="$(topic "$W/p/b" "From B")" || { echo "$out"; exit 1; }
printf '%s' "$out" | grep -q -e '"pulled": 1' || { echo "$out"; exit 1; }
test -f "$W/p/b/topics/from-a.md" && test -f "$W/p/b/topics/from-b.md"
test "$(git -C "$W/p/b" rev-list --merges --count HEAD)" = 0
CMD
expect_ok "unreachable remote: run continues with PULL_FAILED warning" <<'CMD'
remote_pair "$W/c" || exit 1
git -C "$W/c/b" remote set-url origin "$W/c/missing.git" || exit 1
out="$(topic "$W/c/b" "Local")" || { echo "$out"; exit 1; }
printf '%s' "$out" | grep -q -e 'PULL_FAILED' && test -f "$W/c/b/topics/local.md"
CMD
expect_fail "negative: git.require_pull with unreachable remote fails PULL_FAILED, nothing written" <<'CMD'
remote_pair "$W/r" || exit 125
printf '%s\n' '{"git":{"require_pull":true}}' > "$W/r/b/.slowhunch/config.json"
git -C "$W/r/b" commit -qam cfg || exit 125
git -C "$W/r/b" remote set-url origin "$W/r/missing.git" || exit 125
head="$(git -C "$W/r/b" rev-parse HEAD)"
fails_with PULL_FAILED topic "$W/r/b" "Blocked"; rc=$?
[ "$head" = "$(git -C "$W/r/b" rev-parse HEAD)" ] && test ! -e "$W/r/b/topics/blocked.md" || exit 125
exit "$rc"
CMD
expect_fail "negative: same-id topic in both clones fails CONFLICT (exit 4), B unchanged" <<'CMD'
remote_pair "$W/k" || exit 125
topic "$W/k/a" "Rivers" >/dev/null && git -C "$W/k/a" push -q 2>/dev/null || exit 125
"$SH" --vault "$W/k/b" --json --no-pull create topic --data '{"title":"Rivers","body":"other text"}' >/dev/null || exit 125
head="$(git -C "$W/k/b" rev-parse HEAD)"; before="$(treehash "$W/k/b")"
fails_with CONFLICT topic "$W/k/b" "Other"; rc=$?
[ "$rc" -eq 4 ] || exit 125
[ "$head" = "$(git -C "$W/k/b" rev-parse HEAD)" ] && [ "$before" = "$(treehash "$W/k/b")" ] || exit 125
test ! -e "$W/k/b/.git/rebase-merge" && test ! -e "$W/k/b/.git/rebase-apply" && test ! -e "$W/k/b/topics/other.md" || exit 125
exit "$rc"
CMD
finish
