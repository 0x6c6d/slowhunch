#!/usr/bin/env bash
# Verification for T006 (supervisor-owned; workers never edit it). Exit 0 = pass.
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
expect_ok "TestPromptCollide and provider packages" <<'CMD'
gotest ./internal/provider/... '.*' TestPromptCollide
CMD
expect_ok "cli package" <<'CMD'
go test ./internal/cli/ -count=1
CMD
expect_ok "collide prompt frontmatter and JSON keys" <<'CMD'
f=internal/provider/prompts/collide.md
grep -q -e '^role: collide$' $f && grep -q -e '^version: collide-1$' $f && grep -q -e 'idea' $f && grep -q -e 'cheapest_test' $f && grep -q -e 'keep' $f
CMD
expect_ok "init writes the collide prompt" <<'CMD'
"$SH" init "$W/v" >/dev/null && test -f "$W/v/.slowhunch/prompts/collide.md"
CMD
finish
