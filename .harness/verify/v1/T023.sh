#!/usr/bin/env bash
# Verification for T023 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "lint tests" <<'CMD'
gotest "./internal/vault/... ./internal/cli/..." 'TestLintRules|TestLintCommand' TestLintRules TestLintCommand
CMD
expect_ok "binary: fresh vault lints clean" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 1
"$t/sh" init "$t/v" >/dev/null || exit 1
"$t/sh" lint --vault "$t/v" --json | grep -q '"failures": \[\]'
CMD
expect_fail "negative: confidence on a crafts note fails lint with exit 3, file unchanged" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
cat > "$t/v/notes/crafts-x-line-1.md" <<'EOF'
---
id: crafts-x-line-1
type: note
created: 2026-11-05
updated: 2026-11-05
author: human
rev: abcdef
claim: A claim
evidence: some words
stance: crafts
locator: line 1
confidence: "0.5"
---
A claim

> some words
EOF
cp "$t/v/notes/crafts-x-line-1.md" "$t/before"
out="$("$t/sh" lint --vault "$t/v" --json)"; rc=$?
cmp -s "$t/before" "$t/v/notes/crafts-x-line-1.md" || exit 0
[ "$rc" -eq 3 ] && grep -q 'CONFIDENCE_NOT_ASSERTS' <<<"$out" && grep -q 'crafts-x-line-1' <<<"$out" && exit 1
exit 0
CMD
finish
