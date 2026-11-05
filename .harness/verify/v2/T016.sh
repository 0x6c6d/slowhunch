#!/usr/bin/env bash
# Verification for T016 (supervisor-owned; workers never edit it). Exit 0 = pass.
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
expect_ok "collide command tests" <<'CMD'
gotest ./internal/cli/ 'TestCollide' TestCollideRun TestCollideRerun TestCollideDryRun TestCollideHostileReply
CMD
expect_ok "synth, report and cli packages" <<'CMD'
go test ./internal/synth/ ./internal/report/ ./internal/cli/ -count=1
CMD
expect_ok "collide --dry-run on a fresh vault changes nothing" <<'CMD'
"$SH" init "$W/v" >/dev/null || exit 1
printf '%s\n' '{"provider":{"name":"exec","command":["false"]}}' > "$W/v/.slowhunch/config.json"
git -C "$W/v" commit -qam cfg || exit 1
head="$(git -C "$W/v" rev-parse HEAD)"; before="$(treehash "$W/v")"
"$SH" --vault "$W/v" --json collide --dry-run >/dev/null || exit 1
[ "$head" = "$(git -C "$W/v" rev-parse HEAD)" ] && [ "$before" = "$(treehash "$W/v")" ]
CMD
expect_fail "negative: collide --limit 0 fails BAD_INPUT" <<'CMD'
"$SH" init "$W/n" >/dev/null || exit 125
fails_with BAD_INPUT "$SH" --vault "$W/n" --json collide --limit 0
CMD
expect_fail "negative: proposal paths never come from raw model text" <<'CMD'
grep -rn --include=*.go -e 'filepath.Join(.*[Ii]dea' internal/synth/ internal/cli/ | grep -v -e '_test\.go:'
CMD
finish
