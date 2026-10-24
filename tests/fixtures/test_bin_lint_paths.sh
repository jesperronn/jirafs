#!/usr/bin/env bash
# bin/lint accepts any number of paths, skips non-Go files, and supports
# --autofix with several paths. Runs against a throwaway Go module.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
cd "${tmp}"
mkdir bin one two
cp "${REPO_ROOT}/bin/lint" bin/lint
printf 'module example.com/t\n\ngo 1.21\n' > go.mod
printf 'package one\n\nfunc A() int { return 1 }\n' > one/a.go
printf 'package two\n\nfunc B() int { return 2 }\n' > two/b.go
printf '# Title\n\nSome docs.\n' > README.md

out="$(bin/lint one two README.md 2>&1)" || { echo "multi-path should pass: ${out}"; exit 1; }
case "${out}" in *"lint: ok (one two README.md)"*) ;; *) echo "missing summary: ${out}"; exit 1;; esac
case "${out}" in *"illegal character"*) echo "go parsed markdown: ${out}"; exit 1;; esac
case "${out}" in *"skipping README.md"*) ;; *) echo "missing skip note: ${out}"; exit 1;; esac

# One bad file among several -> non-zero.
printf 'package two\n\nfunc C() int {\n\treturn 3 }\n' > two/c.go
if out="$(bin/lint one two/c.go README.md 2>&1)"; then
    echo "bad file should fail: ${out}"; exit 1
fi
case "${out}" in *"two/c.go"*) ;; *) echo "bad file not named: ${out}"; exit 1;; esac

# Unknown path -> non-zero.
if bin/lint one missing 2>/dev/null; then echo "missing path should fail"; exit 1; fi

# --autofix with two paths.
printf 'package one\n\nfunc D() int {\n\treturn 4 }\n' > one/d.go
bin/lint --autofix one two >/dev/null 2>&1 || { echo "autofix should pass"; exit 1; }
bin/lint one two >/dev/null 2>&1 || { echo "tree should be clean after autofix"; exit 1; }
[ -z "$(gofmt -l one two)" ] || { echo "files still unformatted"; exit 1; }

# --help goes to stdout.
out="$(bin/lint --help 2>/dev/null)"
case "${out}" in *"PATH..."*) ;; *) echo "help not on stdout: ${out}"; exit 1;; esac
