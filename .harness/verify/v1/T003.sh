#!/usr/bin/env bash
# Verification for T003 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "config tests" <<'CMD'
gotest "./internal/vault/..." 'TestConfig' TestConfigDefaults TestConfigPartialFile TestConfigInvalid
CMD
expect_ok "minimal init test" <<'CMD'
gotest "./internal/cli/..." 'TestInitMinimal' TestInitMinimal
CMD
expect_ok "phase 1 exit: make check, bin ignored, init via binary" <<'CMD'
make check && git check-ignore -q bin/x || exit 1
d=$(mktemp -d); trap 'rm -r -- "$d"' EXIT
./bin/slowhunch init "$d/v" --json | grep -q '"schema": *1' && test -d "$d/v/.slowhunch"
CMD
expect_ok "config.json carries the spec defaults and all folders exist" <<'CMD'
d=$(mktemp -d); trap 'rm -r -- "$d"' EXIT
go run ./cmd/slowhunch init "$d/v" >/dev/null || exit 1
c="$d/v/.slowhunch/config.json"
for k in '"name": "openai-compatible"' '"api_key_env": "SLOWHUNCH_API_KEY"' '"atoms_per_asserts_input": 5' '"atoms_per_aesthetic_input": 3' '"chars_per_call": 12000' '"near": 5' '"duplicate_cosine": 0.8' '"contradict_cosine": 0.2' '"excerpt_chars": 300' '"send_policy": "excerpt_only"' '"commit": true'; do
  grep -qF "$k" "$c" || { echo "missing $k"; exit 1; }
done
for f in notes hunches sources topics proposals inbox archive .slowhunch; do test -d "$d/v/$f" || exit 1; done
CMD
expect_fail "negative: init without a directory is a usage error" <<'CMD'
go build -o /dev/null ./cmd/slowhunch || exit 125
out="$(go run ./cmd/slowhunch init --json 2>/dev/null)"; rc=$?
[ "$rc" -eq 0 ] && exit 0
grep -q '"code": "BAD_INPUT"' <<<"$out" && exit 1
exit 125
CMD
finish
