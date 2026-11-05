#!/usr/bin/env bash
# Verification for T010 (supervisor-owned; workers never edit it). Exit 0 = pass.
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
expect_ok "TestPDFExtract and TestPDFUnsupported" <<'CMD'
gotest ./internal/intake/ '.*' TestPDFExtract TestPDFUnsupported
CMD
expect_ok "TestPDFAdd and cli package" <<'CMD'
gotest ./internal/cli/ '.*' TestPDFAdd
CMD
expect_ok "pipeline package" <<'CMD'
go test ./internal/pipeline/ -count=1
CMD
expect_fail "negative: missing pdf.command fails UNSUPPORTED with hint, nothing written" <<'CMD'
"$SH" init "$W/v" >/dev/null || exit 125
printf '%s\n' '{"provider":{"name":"exec","command":["false"]},"pdf":{"command":["slowhunch-no-such-pdf-tool","{file}"]}}' > "$W/v/.slowhunch/config.json"
git -C "$W/v" commit -qam cfg || exit 125
printf '%%PDF-1.4\n' > "$W/p.pdf"
head="$(git -C "$W/v" rev-parse HEAD)"; before="$(treehash "$W/v")"
out="$("$SH" --vault "$W/v" --json add "$W/p.pdf" --title T --author A 2>&1)"; rc=$?
printf '%s' "$out" | grep -q -e 'install pdftotext or set pdf.command' || exit 125
[ "$head" = "$(git -C "$W/v" rev-parse HEAD)" ] && [ "$before" = "$(treehash "$W/v")" ] || exit 125
fails_with UNSUPPORTED "$SH" --vault "$W/v" --json add "$W/p.pdf" --title T --author A
CMD
expect_fail "negative: failing pdf.command (false) fails UNSUPPORTED" <<'CMD'
"$SH" init "$W/f" >/dev/null || exit 125
printf '%s\n' '{"provider":{"name":"exec","command":["false"]},"pdf":{"command":["false"]}}' > "$W/f/.slowhunch/config.json"
git -C "$W/f" commit -qam cfg || exit 125
printf '%%PDF-1.4\n' > "$W/q.pdf"
fails_with UNSUPPORTED "$SH" --vault "$W/f" --json add "$W/q.pdf" --title T --author A
CMD
finish
