#!/usr/bin/env bash

set -euo pipefail

# Verify that required bin scripts exist and are executable.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

for script in bin/lint bin/test bin/verify bin/handoff bin/verify-harness; do
    path="${REPO_ROOT}/${script}"
    if [ ! -f "${path}" ]; then
        echo "FAIL: ${script} missing" >&2
        exit 1
    fi
    if [ ! -x "${path}" ]; then
        echo "FAIL: ${script} is not executable" >&2
        exit 1
    fi
    # Verify shebang is present
    first_line="$(head -n 1 "${path}")"
    case "${first_line}" in
        "#!"*) ;;
        *) echo "FAIL: ${script} missing shebang" >&2; exit 1 ;;
    esac
done

exit 0
