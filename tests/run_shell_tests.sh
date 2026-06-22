#!/usr/bin/env bash

set -euo pipefail

# Discover and run shell-test fixtures in tests/fixtures/
# Reports [PASS] or [FAIL] per fixture and exits non-zero on any failure.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

FIXTURES_DIR="${REPO_ROOT}/tests/fixtures"
PASS=0
FAIL=0

for test_file in "${FIXTURES_DIR}"/*.sh; do
    [ -f "${test_file}" ] || continue
    if bash "${test_file}"; then
        echo "[PASS] $(basename "${test_file}")"
        PASS=$((PASS + 1))
    else
        echo "[FAIL] $(basename "${test_file}")"
        FAIL=$((FAIL + 1))
    fi
done

if [ "${FAIL}" -gt 0 ]; then
    echo "shell tests: ${PASS} passed, ${FAIL} failed" >&2
    exit 1
fi

echo "shell tests: all ${PASS} passed"
exit 0
