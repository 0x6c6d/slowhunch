#!/usr/bin/env bash
# Verification for T012 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "wildcard tests" <<'CMD'
out="$(go test ./internal/... -count=1 -v -run 'TestWildcard' 2>&1)" || { printf '%s\n' "$out" | tail -n 30; exit 1; }
for t in TestWildcardTen TestWildcardSingle TestWildcardShare; do
  printf '%s\n' "$out" | grep -qE "^--- PASS: $t " || { echo "missing or not passing: $t"; exit 1; }
done
CMD
expect_ok "pipeline, report and cli packages" <<'CMD'
go test ./internal/pipeline/ ./internal/report/ ./internal/cli/ -count=1
CMD
expect_ok "Wildcard report type" <<'CMD'
grep -q -E -e 'type Wildcard struct' internal/report/report.go && grep -q -e '"kept"' internal/report/report.go
CMD
expect_fail "negative: wildcards is no longer an always-empty []any" <<'CMD'
grep -n -E -e 'Wildcards +\[\]any' internal/report/report.go
CMD
finish
