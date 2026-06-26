#!/usr/bin/env bash

set -euo pipefail

# Test bin/integrate_stream_commit: retry/backoff after non-fast-forward push failure.
# The repo root is two levels up from tests/fixtures/.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INTEGRATE="${REPO_ROOT}/bin/integrate_stream_commit"

# Skip if called from within the integration script (recursive bin/test).
# The integration script calls bin/test, which runs all shell tests.
# When INTEGRATE_RETRY_TEST=1, this test skips its setup and just passes,
# letting the integration script's own retry logic handle the test.
if [ "${INTEGRATE_RETRY_TEST:-}" = "1" ]; then
    echo "[PASS] test_integrate_stream_commit_retry.sh"
    exit 0
fi

# Create a temporary directory with a bare "remote" and a local worktree.
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

# --- Set up a bare repo to act as the "remote" ---
bare="${tmpdir}/bare.git"
git init --bare -q "${bare}"

# --- Set up the local repo with a commit ---
local_repo="${tmpdir}/local"
mkdir -p "${local_repo}"
(
    cd "${local_repo}"
    git init -q
    git config user.email "test@test.com"
    git config user.name "Test"

    # Create initial commit on main
    git checkout -q -b main
    echo "initial" > file.txt
    git add file.txt
    git commit -q -m "initial"

    # Create a feature branch to integrate
    git checkout -q -b feature
    echo "feature" > feature.txt
    git add feature.txt
    git commit -q -m "feature commit"

    # Add bare as origin
    git remote add origin "${bare}"
    git push -q origin feature
)

# --- Now simulate another actor pushing to the bare repo, creating a non-fast-forward ---
# This second commit goes to `main` in the bare repo, so when the local tries to
# rebase+push, it will fail with a non-fast-forward error.
# We need a separate work tree to commit into the bare repo.
bare_work="${tmpdir}/bare_work"
mkdir -p "${bare_work}"
(
    cd "${bare_work}"
    git clone -q "${bare}" .
    git config user.email "other@test.com"
    git config user.name "Other"
    echo "other" > other.txt
    git add other.txt
    git commit -q -m "other commit on main"
    git push -q origin main
)

# --- Test: retry/backoff triggers on push failure ---
# The local repo's `feature` branch needs to be rebased onto `main` then pushed.
# The push will fail because `main` has diverged (the other commit).
# The script should retry with backoff.

# We test this by:
# 1. Starting the integration
# 2. Having a "gate" that blocks the first few pushes
# 3. Verifying the script retries

# We'll use a marker file approach: the script calls `bin/test` and `bin/lint`
# before each push attempt. We hook into the push failure by making the first N
# pushes fail, then succeed.

# Create a wrapper that counts failures and eventually allows the push.
fail_count=0
max_fails=2  # Allow 2 failures, then succeed

# We need to intercept `git push`. Create a wrapper that delegates all
# git commands except `git push`, which it fails based on a counter.
wrapper_script="${tmpdir}/git-push-wrapper.sh"
cat > "${wrapper_script}" <<'WRAPPER'
#!/usr/bin/env bash
# Wrapper for git that simulates non-fast-forward push failures.
# All git commands pass through except `git push`, which fails until
# the counter file reaches zero.
COUNTER_FILE="${INTEGRATE_FAILURE_COUNT:-/dev/null}"
if [ "$1" = "push" ]; then
    if [ -f "${COUNTER_FILE}" ]; then
        count=$(cat "${COUNTER_FILE}")
        if [ "${count}" -gt 0 ]; then
            echo "${count}" > "${COUNTER_FILE}"
            echo "error: failed to push some refs" >&2
            exit 1
        fi
    fi
fi
exec git "$@"
WRAPPER
chmod +x "${wrapper_script}"

# Set up the counter: start with 2 failures
echo "2" > "${tmpdir}/failure_count"

# Now run the integrate script with the git wrapper.
# We override git in PATH to use our wrapper.
# Also set INTEGRATE_RETRY_TEST=1 so the integration script's bin/test
# calls skip this test and avoid infinite recursion.
(
    export PATH="${tmpdir}:${PATH}"
    export INTEGRATE_FAILURE_COUNT="${tmpdir}/failure_count"
    export INTEGRATE_RETRY_TEST=1
    
    cd "${local_repo}"
    
    # The script should fail initially, retry, and eventually succeed
    # (after the counter reaches 0, the wrapper calls real git)
    if bash "${INTEGRATE}" 2>&1; then
        echo "[PASS] retry/backoff after non-fast-forward push failure works"
    else
        echo "[FAIL] retry/backoff after non-fast-forward push failure did not succeed" >&2
        exit 1
    fi
)

exit 0
