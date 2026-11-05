#!/usr/bin/env bash
# Verification for T023 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "README exists with the core headings" <<'CMD'
f=README.md
test -f $f || exit 1
for h in '## Requirements' '## Installation' '## Quick start' '## Usage' '## Configuration' '## Project structure' '## Development'; do
  grep -E '^## ' $f | grep -q -x -F -e "$h" || { echo "missing heading: $h"; exit 1; }
done
head -n 1 $f | grep -q -e '^# slowhunch'
CMD
expect_fail "negative: no .harness, TODO or TBD in README" <<'CMD'
grep -n -e '\.harness' -e 'TODO' -e 'TBD' README.md
CMD
expect_fail "negative: no <...> placeholder text outside code literals" <<'CMD'
grep -n -E -e '<\.\.\.>' -e '<(your|insert|placeholder|replace)[^>]*>' README.md
CMD
expect_ok "README names every new command and config key" <<'CMD'
for k in 'slowhunch collide' 'slowhunch review' 'slowhunch keep' '--from' 'slowhunch sync' 'pdf.command' 'pdftotext' 'media' 'caps.by_medium' 'runs_per_day' 'tokens_per_run' 'git.pull' 'git.push' 'git.require_pull' 'git.remote' 'git.branch' 'collide.max_cosine' 'collide.aesthetic_weight' 'collide.max_pairs' 'review.stale_days' 'place.wildcard_share' '--no-pull' '--no-push' '--sync' 'CAP_REACHED' 'PULL_FAILED' 'PUSH_FAILED' 'BEHIND' 'internal/synth/'; do
  grep -q -F -e "$k" README.md || { echo "README lacks: $k"; exit 1; }
done
CMD
expect_ok "clean temp copy: from-source install and Quick start test" <<'CMD'
t="$(mktemp -d)"; trap 'rm -rf "$t"' EXIT
git ls-files -z --cached --others --exclude-standard | tar --null -T - -cf - | tar -xf - -C "$t" || exit 1
cd "$t" || exit 1
timeout 300 make build >/dev/null || exit 1
test -x bin/slowhunch || exit 1
./bin/slowhunch help >/dev/null || exit 1
out="$(timeout 300 go test ./internal/cli/ -count=1 -v -run 'TestReadme' 2>&1)" || { printf '%s\n' "$out" | tail -n 30; exit 1; }
printf '%s\n' "$out" | grep -qE '^--- PASS: TestReadmeQuickstart ' && printf '%s\n' "$out" | grep -qE '^--- PASS: TestReadmeV2Mentions '
CMD
finish
