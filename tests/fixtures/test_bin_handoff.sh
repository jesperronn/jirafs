#!/usr/bin/env bash
# bin/handoff must rebase + verify only and never move main.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
cd "${tmp}"
git init -q -b main .
git config user.email t@example.com
git config user.name t
mkdir bin
cp "${REPO_ROOT}/bin/handoff" bin/handoff
printf '#!/usr/bin/env bash\nexit 0\n' > bin/verify
chmod +x bin/handoff bin/verify
echo a > a; git add -A; git commit -qm base

out="$(bin/handoff 2>&1 || true)"
case "${out}" in blocked:*main*) ;; *) echo "expected block on main: ${out}"; exit 1;; esac

git checkout -q -b feat
out="$(bin/handoff)"
case "${out}" in clean:*) ;; *) echo "expected clean: ${out}"; exit 1;; esac

echo b > b; git add -A; git commit -qm feat
git checkout -q main; echo c > c; git add -A; git commit -qm other
git checkout -q feat
before="$(git rev-parse main)"
out="$(bin/handoff)"
after="$(git rev-parse main)"
[ "${before}" = "${after}" ] || { echo "main moved"; exit 1; }
case "${out}" in done:*"git merge --ff-only feat"*) ;; *) echo "bad done line: ${out}"; exit 1;; esac
git merge-base --is-ancestor main feat || { echo "feat not rebased"; exit 1; }

# conflict -> blocked, tree clean, main unchanged
git checkout -q main; echo m > f; git add -A; git commit -qm m
git checkout -q feat; echo x > f; git add -A; git commit -qm x
before="$(git rev-parse main)"
out="$(bin/handoff 2>&1 || true)"
case "${out}" in blocked:*) ;; *) echo "expected blocked: ${out}"; exit 1;; esac
[ "$(git rev-parse main)" = "${before}" ] || { echo "main moved on conflict"; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "tree dirty"; exit 1; }
