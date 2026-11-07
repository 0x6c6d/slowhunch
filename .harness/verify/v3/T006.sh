#!/usr/bin/env bash
# Verification for T006 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "intake unit tests [R41][R42]" <<'CMD'
gotest ./internal/intake/ 'Wikilink' TestWikilinkStrip TestWikilinkCodeUntouched TestWikilinkHash
CMD
expect_ok "e2e tests [R41][R42]" <<'CMD'
gotest ./internal/cli/ 'Wikilink' TestAddWikilinkVerifyClean TestAddRawWikilinkEvidenceRejected
CMD
expect_ok "legacy and pipe tests [R41][R42]" <<'CMD'
OUT=$(mktemp)
go test ./internal/vault/ ./internal/frontmatter/ -count=1 -v -run 'WikilinkUnchanged|PipeRoundTrip' | tee "$OUT"
grep -q -e '--- PASS: TestVerifyLegacyWikilinkUnchanged' "$OUT"
grep -q -e '--- PASS: TestEvidencePipeRoundTrip' "$OUT"
CMD
expect_ok "README check" <<'CMD'
bash .harness/verify/v3/T004.sh
CMD
expect_ok "make check" <<'CMD'
make check
CMD
finish
