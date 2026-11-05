#!/usr/bin/env bash
# Verification for T022 (supervisor-owned; workers never edit it). Exit 0 = pass.
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
expect_ok "skill tests" <<'CMD'
gotest ./internal/cli/ 'TestSkill' TestSkill TestSkillV2
CMD
expect_ok "skill output fits 60 lines and names the new commands" <<'CMD'
out="$("$SH" skill)" || exit 1
[ "$(printf '%s\n' "$out" | wc -l)" -le 60 ] || { echo "too many lines"; exit 1; }
for k in 'slowhunch collide' 'slowhunch review' 'slowhunch keep' '--from' 'slowhunch sync'; do
  printf '%s' "$out" | grep -q -F -e "$k" || { echo "missing: $k"; exit 1; }
done
CMD
finish
