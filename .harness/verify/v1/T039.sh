#!/usr/bin/env bash
# Verification for T039 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "add CLI tests" <<'CMD'
gotest "./internal/cli/..." 'TestAddCLI' TestAddCLIArticle TestAddCLIPoem TestAddCLIErrors TestAddCLIDryRun TestAddCLIConflict
CMD
expect_ok "phase 6 exit pattern" <<'CMD'
go test ./internal/pipeline/... ./internal/cli/... -count=1 -run 'TestAdd|TestGate|TestExtract|TestPlace|TestReport|TestDryRun|TestIdempotent|TestResume|TestFencing|TestSecret'
CMD
expect_fail "negative: a .pdf input exits 2 UNSUPPORTED and writes nothing" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
go build -o "$t/sh" ./cmd/slowhunch || exit 125
"$t/sh" init "$t/v" >/dev/null || exit 125
printf '%%PDF-1.4\n' > "$t/paper.pdf"
out="$("$t/sh" add "$t/paper.pdf" --vault "$t/v" --json)"; rc=$?
[ -n "$(find "$t/v/inbox" "$t/v/sources" -type f)" ] && exit 0
[ "$rc" -eq 2 ] && grep -q '"code": "UNSUPPORTED"' <<<"$out" && exit 1
exit 0
CMD
expect_ok "no double-dash force literal in non-test Go files" <<'CMD'
test -z "$(git grep -nE 'git (push|pull)|--force' -- '*.go' ':!*_test.go'; grep -rnE --include='*.go' 'git (push|pull)|--force' internal cmd | grep -v '_test\.go:')"
CMD
finish
