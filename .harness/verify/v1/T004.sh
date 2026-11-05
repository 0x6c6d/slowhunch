#!/usr/bin/env bash
# Verification for T004 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "TestSkill" <<'CMD'
gotest "./internal/cli/..." 'TestSkill' TestSkill
CMD
expect_ok "skill output is 20 to 60 lines with the key commands" <<'CMD'
out="$(go run ./cmd/slowhunch skill)" || exit 1
n=$(printf '%s\n' "$out" | wc -l)
[ "$n" -ge 20 ] && [ "$n" -le 60 ] || { echo "lines: $n"; exit 1; }
for k in 'slowhunch verify' 'slowhunch add' 'slowhunch search' '--json' '[[id]]' 'inbox/'; do grep -qF -- "$k" <<<"$out" || { echo "missing $k"; exit 1; }; done
CMD
expect_ok "skill JSON mode" <<'CMD'
out="$(go run ./cmd/slowhunch skill --json)" && grep -q '"schema": 1' <<<"$out" && grep -q '"text":' <<<"$out"
CMD
expect_fail "negative: guide never contains the double-dash force flag" <<'CMD'
go build -o /dev/null ./cmd/slowhunch || exit 125
go run ./cmd/slowhunch skill | grep -qF -- '--force'
CMD
finish
