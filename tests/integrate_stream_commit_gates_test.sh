#!/usr/bin/env bash

set -euo pipefail

# Test bin/integrate_stream_commit: rebase, test, and lint failure paths stop
# before push.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INTEGRATE="${REPO_ROOT}/bin/integrate_stream_commit"

# --- Test: rebase failure stops before push ---
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

(
    cd "${tmpdir}"
    git init -q
    git config user.email "test@test.com"
    git config user.name "Test"

    # Create initial commit on main
    git checkout -q -b main
    echo "initial" > file.txt
    git add file.txt
    git commit -q -m "initial"

    # Create a feature branch
    git checkout -q -b feature
    echo "feature" > feature.txt
    git add feature.txt
    git commit -q -m "feature commit"

    # Make main and feature diverge by adding a commit to main from another "actor"
    git checkout -q main
    echo "other" > other.txt
    git add other.txt
    git commit -q -m "other commit"

    # Now go back to feature and try to integrate — rebase will fail
    git checkout -q feature

    # Create a fake git that rewrites 'rebase' to fail
    fakebin="${tmpdir}/fakebin"
    mkdir -p "${fakebin}"
    cat > "${fakebin}/git" <<'FAKEGIT'
#!/usr/bin/env bash
if [ "$1" = "rebase" ]; then
    echo "error: could not rebase" >&2
    exit 1
fi
exec git "$@"
FAKEGIT
    chmod +x "${fakebin}/git"

    export PATH="${fakebin}:${PATH}"

    # The script should fail on rebase failure, before attempting push
    if INTEGRATE_RETRY_TEST=1 bash "${INTEGRATE}" 2>/dev/null; then
        echo "[FAIL] rebase failure should stop before push" >&2
        exit 1
    else
        echo "[PASS] rebase failure stops before push"
    fi
)

# --- Test: test failure stops before push ---
tmpdir2="$(mktemp -d)"
trap 'rm -rf "${tmpdir2}"' EXIT

(
    cd "${tmpdir2}"
    git init -q
    git config user.email "test@test.com"
    git config user.name "Test"

    # Create initial commit on main
    git checkout -q -b main
    echo "initial" > file.txt
    git add file.txt
    git commit -q -m "initial"

    # Create a feature branch
    git checkout -q -b feature
    echo "feature" > feature.txt
    git add feature.txt
    git commit -q -m "feature commit"

    # Create a fake bin/test that always fails
    fakebin2="${tmpdir2}/fakebin2"
    mkdir -p "${fakebin2}"
    cat > "${fakebin2}/bin" <<'FAKEBIN'
#!/usr/bin/env bash
echo "fake test failure" >&2
exit 1
FAKEBIN
    mkdir -p "${fakebin2}/bin"
    cat > "${fakebin2}/bin/test" <<'FAKETEST'
#!/usr/bin/env bash
echo "fake test failure" >&2
exit 1
FAKETEST
    chmod +x "${fakebin2}/bin/test"

    # Create a fake bin/lint that passes (we only want test to fail)
    mkdir -p "${fakebin2}/bin"
    cat > "${fakebin2}/bin/lint" <<'FAKELINT'
#!/usr/bin/env bash
exit 0
FAKELINT
    chmod +x "${fakebin2}/bin/lint"

    export PATH="${fakebin2}:${PATH}"

    # The script should fail on test failure, before attempting push
    if INTEGRATE_RETRY_TEST=1 bash "${INTEGRATE}" 2>/dev/null; then
        echo "[FAIL] test failure should stop before push" >&2
        exit 1
    else
        echo "[PASS] test failure stops before push"
    fi
)

# --- Test: lint failure stops before push ---
tmpdir3="$(mktemp -d)"
trap 'rm -rf "${tmpdir3}"' EXIT

(
    cd "${tmpdir3}"
    git init -q
    git config user.email "test@test.com"
    git config user.name "Test"

    # Create initial commit on main
    git checkout -q -b main
    echo "initial" > file.txt
    git add file.txt
    git commit -q -m "initial"

    # Create a feature branch
    git checkout -q -b feature
    echo "feature" > feature.txt
    git add feature.txt
    git commit -q -m "feature commit"

    # Create a fake bin/test that passes
    fakebin3="${tmpdir3}/fakebin3"
    mkdir -p "${fakebin3}/bin"
    cat > "${fakebin3}/bin/test" <<'FAKETEST'
#!/usr/bin/env bash
exit 0
FAKETEST
    chmod +x "${fakebin3}/bin/test"

    # Create a fake bin/lint that always fails
    cat > "${fakebin3}/bin/lint" <<'FAKELINT'
#!/usr/bin/env bash
echo "fake lint failure" >&2
exit 1
FAKELINT
    chmod +x "${fakebin3}/bin/lint"

    export PATH="${fakebin3}:${PATH}"

    # The script should fail on lint failure, before attempting push
    if INTEGRATE_RETRY_TEST=1 bash "${INTEGRATE}" 2>/dev/null; then
        echo "[FAIL] lint failure should stop before push" >&2
        exit 1
    else
        echo "[PASS] lint failure stops before push"
    fi
)

echo "[PASS] test_integrate_stream_commit_gates_test.sh"
exit 0
