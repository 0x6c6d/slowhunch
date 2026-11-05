#!/usr/bin/env bash
# Shared verification helpers for .harness/verify/vN/<ID>.sh (supervisor-owned; workers never edit it).
#
#   expect_ok   "<label>" <<'CMD' ... CMD   command must exit 0
#   expect_fail "<label>" <<'CMD' ... CMD   command must exit with an ordinary failure (1..123);
#                                           0, a timeout (124), a missing command (126/127) or a
#                                           signal (>128) count as a failed check
#   finish                                  prints the summary, exits 1 if any check failed
#
# Each command runs with `bash -c` from the repository root under `timeout $VERIFY_TIMEOUT`
# (default 600 s; override per check with `VERIFY_TIMEOUT=900 expect_ok ...`).
# Helpers exported into every command:
#   gotest "<packages>" "<run-regex>" [TestName ...]
#       go test -count=1 -v; fails unless go test exits 0 AND every named top-level test
#       printed "--- PASS: <TestName>" (so a missing test is a failure, not "no tests to run").
#   goprobe
#       reads a Go `package main` program on stdin, builds it as a throw-away package
#       internal/zzprobe*/ inside the module (so it may import internal packages), runs it and
#       removes it. Build failure or a panic returns 125 (never an "expected failure");
#       otherwise the program's exit code is returned. Probes use only os.Exit(0) / os.Exit(1).

VERIFY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export VERIFY_ROOT
: "${VERIFY_TIMEOUT:=600}"
_V_PASS=0
_V_FAIL=0

gotest() {
  local pkgs="$1" run="$2" out rc name
  shift 2
  # shellcheck disable=SC2086
  out="$(go test $pkgs -count=1 -v -run "$run" 2>&1)"
  rc=$?
  if [ "$rc" -ne 0 ]; then
    printf '%s\n' "$out" | grep -E -e '^(--- FAIL|FAIL|panic)' -e '_test\.go:[0-9]+' -e 'cannot|undefined|error' | tail -n 40
    return "$rc"
  fi
  for name in "$@"; do
    if ! printf '%s\n' "$out" | grep -qE "^--- PASS: ${name} "; then
      echo "missing or not passing: ${name}"
      return 1
    fi
  done
  return 0
}

goprobe() {
  local d rc
  d="$(mktemp -d "$VERIFY_ROOT/internal/zzprobeXXXXXX")" || return 125
  cat >"$d/main.go"
  if ! (cd "$VERIFY_ROOT" && go build -o "$d/probe.bin" "./internal/$(basename "$d")") >"$d/build.log" 2>&1; then
    tail -n 20 "$d/build.log"
    rm -rf "$d"
    return 125
  fi
  (cd "$d" && ./probe.bin)
  rc=$?
  rm -rf "$d"
  [ "$rc" -eq 2 ] && return 125
  return "$rc"
}
export -f gotest goprobe

_v_run() {
  local mode="$1" label="$2" cmd out rc
  cmd="$(cat)"
  out="$(cd "$VERIFY_ROOT" && timeout "$VERIFY_TIMEOUT" bash -c "$cmd" 2>&1)"
  rc=$?
  if [ "$mode" = ok ]; then
    if [ "$rc" -eq 0 ]; then
      _V_PASS=$((_V_PASS + 1)); echo "PASS  $label"; return 0
    fi
  else
    if [ "$rc" -ge 1 ] && [ "$rc" -le 123 ]; then
      _V_PASS=$((_V_PASS + 1)); echo "PASS  $label (failed as expected, exit $rc)"; return 0
    fi
  fi
  _V_FAIL=$((_V_FAIL + 1))
  echo "FAIL  $label (exit $rc)"
  printf '%s\n' "$out" | tail -n 30 | sed 's/^/      /'
  return 0
}

expect_ok() { _v_run ok "$1"; }
expect_fail() { _v_run fail "$1"; }

finish() {
  echo "---- ${_V_PASS} passed, ${_V_FAIL} failed"
  [ "$_V_FAIL" -eq 0 ]
  exit $?
}
