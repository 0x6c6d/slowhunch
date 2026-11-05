#!/usr/bin/env bash
# Verification for T001 (supervisor-owned; workers never edit it). Exit 0 = pass.
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
expect_ok "TestConfigV2 and vault package" <<'CMD'
gotest ./internal/vault/ '.*' TestConfigV2
CMD
expect_ok "init writes v2 config sections" <<'CMD'
"$SH" init "$W/v" >/dev/null || exit 1
c="$W/v/.slowhunch/config.json"
grep -q -e '"max_cosine"' "$c" && grep -q -e '"wildcard_share"' "$c" && grep -q -e 'pdftotext' "$c" && grep -q -e '"require_pull"' "$c" && grep -q -e '"stale_days"' "$c"
CMD
expect_fail "negative: unknown role key fails BAD_INPUT" <<'CMD'
"$SH" init "$W/r" >/dev/null || exit 125
printf '%s\n' '{"roles":{"gatee":{"model":"x"}}}' > "$W/r/.slowhunch/config.json"
fails_with BAD_INPUT "$SH" --vault "$W/r" --json list
CMD
expect_fail "negative: unknown medium under caps.by_medium fails BAD_INPUT" <<'CMD'
"$SH" init "$W/m" >/dev/null || exit 125
printf '%s\n' '{"caps":{"by_medium":{"podcast":{"runs_per_day":1}}}}' > "$W/m/.slowhunch/config.json"
fails_with BAD_INPUT "$SH" --vault "$W/m" --json list
CMD
finish
