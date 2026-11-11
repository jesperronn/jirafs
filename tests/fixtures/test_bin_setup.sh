#!/usr/bin/env bash
# bin/setup --check verifies tools with no side effects; plain run downloads modules.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
cleanup_test_tmp() { rm -rf "${tmp}"; }
trap cleanup_test_tmp EXIT

pass() { echo "[PASS] $1"; }
fail() { echo "[FAIL] $1" >&2; exit 1; }
has() { case "$1" in *"$2"*) ;; *) fail "expected '$2' in: $1" ;; esac; }

proj="${tmp}/proj"
mkdir -p "${proj}/bin" "${tmp}/stubs"
cp "${REPO_ROOT}/bin/setup" "${proj}/bin/setup"
chmod +x "${proj}/bin/setup"
printf 'module x\n\ngo 1.26\n' > "${proj}/go.mod"

# stub NAME BODY: executable stub first on PATH that logs its calls.
stub() { printf '#!/bin/sh\n%s\n' "$2" > "${tmp}/stubs/$1"; chmod +x "${tmp}/stubs/$1"; }
reset_stubs() {
    rm -f "${tmp}"/stubs/* "${tmp}/calls"
    stub go 'echo "go $*" >> '"${tmp}"'/calls; [ "$1" = version ] && echo "go version go${FAKE_GO:-1.27.4} darwin/arm64"; exit 0'
    stub gofmt 'exit 0'
    stub python3 'exit 0'
    stub git 'exit 0'
    # awk, cat and dirname are the only external commands bin/setup needs besides the stubs.
    ln -s "$(command -v awk)" "${tmp}/stubs/awk"
    ln -s "$(command -v dirname)" "${tmp}/stubs/dirname"
    ln -s "$(command -v cat)" "${tmp}/stubs/cat"
}
run() { PATH="${tmp}/stubs" "${BASH}" "${proj}/bin/setup" "$@" 2>&1; }

reset_stubs
rc=0; out="$(run --check)" || rc=$?
[ "${rc}" -eq 0 ] || fail "--check exit ${rc}: ${out}"
has "${out}" "go 1.27.4 (go.mod wants 1.26)"
if grep -q "mod download" "${tmp}/calls"; then fail "--check ran go mod download"; fi
pass "--check passes with all tools and runs no download"

rc=0; out="$(run)" || rc=$?
[ "${rc}" -eq 0 ] || fail "plain run exit ${rc}: ${out}"
grep -q "go mod download" "${tmp}/calls" || fail "go mod download not run"
run >/dev/null
[ "$(grep -c 'mod download' "${tmp}/calls")" -eq 2 ] || fail "second run not repeatable"
pass "plain run downloads modules and is repeatable"

reset_stubs; rm "${tmp}/stubs/python3"
rc=0; out="$(run --check)" || rc=$?
[ "${rc}" -eq 9 ] || fail "missing python3 exit ${rc}"
has "${out}" "MISSING  python3"
pass "missing tool exits 9 and names it"

reset_stubs
rc=0; out="$(FAKE_GO=1.25.9 run --check)" || rc=$?
[ "${rc}" -eq 9 ] || fail "old go exit ${rc}"
has "${out}" "older than 1.26"
pass "go older than go.mod exits 9"

rc=0; run --bogus >/dev/null || rc=$?
[ "${rc}" -eq 2 ] || fail "usage exit ${rc}"
pass "unknown flag exits 2"
