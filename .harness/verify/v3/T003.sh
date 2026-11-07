#!/usr/bin/env bash
# Verification for T003 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "legacy and pipe tests" <<'CMD'
OUT=$(mktemp)
go test ./internal/vault/ ./internal/frontmatter/ -count=1 -v -run 'WikilinkUnchanged|PipeRoundTrip' | tee "$OUT"
grep -q -e '--- PASS: TestVerifyLegacyWikilinkUnchanged' "$OUT"
grep -q -e '--- PASS: TestEvidencePipeRoundTrip' "$OUT"
CMD
expect_fail "negative: no skipped tests" <<'CMD'
grep -rn 't.Skip' internal/vault/wikilink_legacy_test.go
CMD
expect_ok "make check" <<'CMD'
make check
CMD
finish
