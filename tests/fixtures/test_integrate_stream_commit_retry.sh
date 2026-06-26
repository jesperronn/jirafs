#!/usr/bin/env bash

set -euo pipefail

# Test bin/integrate_stream_commit: retry/backoff after push failure.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SOURCE_INTEGRATE="${REPO_ROOT}/bin/integrate_stream_commit"

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

repo="${tmpdir}/repo"
mkdir -p "${repo}/bin"

cp "${SOURCE_INTEGRATE}" "${repo}/bin/integrate_stream_commit"
chmod +x "${repo}/bin/integrate_stream_commit"

cat > "${repo}/bin/test" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
exit 0
EOF
chmod +x "${repo}/bin/test"

cat > "${repo}/bin/lint" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
exit 0
EOF
chmod +x "${repo}/bin/lint"

(
    cd "${repo}"
    git init -q
    git config user.email "test@test.com"
    git config user.name "Test"

    git checkout -q -b main
    echo "initial" > file.txt
    git add file.txt
    git commit -q -m "initial"

    git checkout -q -b feature
    echo "feature" > feature.txt
    git add feature.txt
    git commit -q -m "feature commit"
)

wrapper_dir="${tmpdir}/wrapper"
mkdir -p "${wrapper_dir}"
counter_file="${tmpdir}/failure_count"
echo "2" > "${counter_file}"

cat > "${wrapper_dir}/git" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

COUNTER_FILE="${INTEGRATE_FAILURE_COUNT}"
REAL_GIT="${REAL_GIT}"

if [ "${1:-}" = "push" ]; then
    count="$(cat "${COUNTER_FILE}")"
    if [ "${count}" -gt 0 ]; then
        echo $((count - 1)) > "${COUNTER_FILE}"
        echo "error: failed to push some refs" >&2
        exit 1
    fi
fi

exec "${REAL_GIT}" "$@"
EOF
chmod +x "${wrapper_dir}/git"

(
    export PATH="${wrapper_dir}:${PATH}"
    export INTEGRATE_FAILURE_COUNT="${counter_file}"
    export REAL_GIT="$(command -v git)"

    cd "${repo}"

    if ! bash "${repo}/bin/integrate_stream_commit" >"${tmpdir}/integrate.out" 2>&1; then
        cat "${tmpdir}/integrate.out" >&2
        echo "[FAIL] retry/backoff after push failure did not succeed" >&2
        exit 1
    fi
)

if [ "$(cat "${counter_file}")" != "0" ]; then
    echo "[FAIL] expected push retry counter to be exhausted" >&2
    exit 1
fi

feature_commit="$(cd "${repo}" && git rev-parse feature)"
main_commit="$(cd "${repo}" && git rev-parse main)"
if [ "${feature_commit}" != "${main_commit}" ]; then
    echo "[FAIL] expected main to advance to feature commit after retries" >&2
    exit 1
fi

echo "[PASS] retry/backoff after push failure works"
exit 0
