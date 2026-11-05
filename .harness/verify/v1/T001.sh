#!/usr/bin/env bash
# Verification for T001 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "go.mod module path and go directive, no require" <<'CMD'
grep -qx 'module github.com/0x6c6d/slowhunch' go.mod && grep -qx 'go 1.27' go.mod && ! grep -q 'require' go.mod
CMD
expect_ok "no go.sum" <<'CMD'
test ! -e go.sum
CMD
expect_ok "make check passes and builds bin/slowhunch" <<'CMD'
make check && test -x bin/slowhunch
CMD
expect_ok "bin/ is git-ignored and existing ignore lines kept" <<'CMD'
git check-ignore -q bin/x && grep -qx '.yipyip/' .gitignore && grep -qx '.env' .gitignore
CMD
expect_ok "MIT license" <<'CMD'
head -n 1 LICENSE | grep -qx 'MIT License' && grep -q 'Permission is hereby granted, free of charge' LICENSE
CMD
expect_ok "Makefile has build test lint check targets" <<'CMD'
for t in build test lint check; do grep -qE "^${t}:" Makefile || exit 1; done
CMD
expect_fail "negative: make lint fails on an unformatted Go file" <<'CMD'
mkdir -p internal || exit 125
d="$(mktemp -d internal/zzfmtXXXXXX)" || exit 125
trap 'rm -r -- "$d"; rmdir internal 2>/dev/null' EXIT
printf 'package zz\nfunc  F( ) {  }\n' > "$d/x.go"
make lint >/dev/null 2>&1
CMD
finish
