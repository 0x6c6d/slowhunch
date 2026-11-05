#!/usr/bin/env bash
# Verification for T043 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "README exists with the required headings in order" <<'CMD'
test -f README.md || exit 1
head -n 1 README.md | grep -qx '# slowhunch' || exit 1
want='## Requirements
## Installation
## Quick start
## Usage
## Configuration
## Privacy and copyright
## Project structure
## Development
## License'
[ "$(grep -E '^## ' README.md)" = "$want" ] || { grep -E '^## ' README.md; exit 1; }
CMD
expect_ok "README mentions the key commands, config and privacy" <<'CMD'
for k in 'slowhunch init' 'slowhunch add' 'slowhunch search' 'slowhunch verify' 'slowhunch hunch' 'slowhunch source facts' 'slowhunch stats' 'slowhunch skill' 'config.json' 'SLOWHUNCH_API_KEY' 'base_url' 'excerpt_only' 'private' 'make check' 'MIT'; do
  grep -qF -- "$k" README.md || { echo "missing: $k"; exit 1; }
done
grep -q '^```sh' README.md
CMD
expect_fail "negative: no internal tooling references or placeholder text" <<'CMD'
test -f README.md || exit 125
grep -nE -e '\.harness' -e 'TODO' -e 'TBD' -e '<\.\.\.>' -e '<[a-z_ -]+>' -e 'Tasks\.md' README.md
CMD
expect_ok "TestReadmeQuickstart" <<'CMD'
gotest "./internal/cli/..." 'TestReadmeQuickstart' TestReadmeQuickstart
CMD
expect_ok "installation from a clean temporary copy: make build" <<'CMD'
t=$(mktemp -d); trap 'rm -r -- "$t"' EXIT
git ls-files -co --exclude-standard -z | grep -zv '^\.harness/' | xargs -0 cp --parents -t "$t" || exit 1
cd "$t" && timeout 300 make build && test -x bin/slowhunch && ./bin/slowhunch help >/dev/null
CMD
finish
