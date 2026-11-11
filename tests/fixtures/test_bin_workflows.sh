#!/usr/bin/env bash
# bin/workflows prints an index of bin/ from "# name — text" lines and runs nothing.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
cleanup_test_tmp() { rm -rf "${tmp}"; }
trap cleanup_test_tmp EXIT

pass() { echo "[PASS] $1"; }
fail() { echo "[FAIL] $1" >&2; exit 1; }
has() { case "$1" in *"$2"*) ;; *) fail "${3:-missing}: $2" ;; esac; }

mkdir "${tmp}/bin"
cp "${REPO_ROOT}/bin/workflows" "${tmp}/bin/workflows"
printf '#!/usr/bin/env bash\n# alpha — does alpha things.\nexit 0\n' > "${tmp}/bin/alpha"
printf '#!/usr/bin/env bash\nexit 0\n' > "${tmp}/bin/nodesc"
printf '#!/usr/bin/env bash\n# beta — skipped test.\n' > "${tmp}/bin/beta.test.sh"
chmod +x "${tmp}"/bin/*

out="$("${tmp}/bin/workflows" commands)"
has "${out}" "alpha            does alpha things." "scraped description"; pass "scrapes the description line"
has "${out}" "nodesc           (no description line)" "fallback"; pass "flags a script without a description"
case "${out}" in *beta*) fail "test.sh listed" ;; esac; pass "skips *.test.sh"

all="$("${tmp}/bin/workflows")"
for s in "== commands ==" "== setup ==" "== test ==" "== lint ==" "== release and integration ==" "bin/handoff" "bin/verify"; do
    has "${all}" "${s}" "section"
done
pass "default prints every section, with verify and handoff under release"

"${tmp}/bin/workflows" --help >/dev/null || fail "--help exit"
pass "--help exits 0"

rc=0
"${tmp}/bin/workflows" bogus >/dev/null 2>&1 || rc=$?
[ "${rc}" -eq 2 ] || fail "unknown workflow exit ${rc}"
pass "unknown workflow exits 2"

# Every real bin/ script carries a description line.
real="$("${REPO_ROOT}/bin/workflows" commands)"
case "${real}" in *"(no description line)"*) fail "a repo bin/ script lacks '# name — text'" ;; esac
pass "every repo bin/ script has a description line"
