#!/usr/bin/env bash
# Install a snapshot by default or explicitly link the checkout launcher safely.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
pass() { echo "[PASS] $1"; }
fail() { echo "[FAIL] $1" >&2; exit 1; }
proj="$tmp/checkout space"
prefix="$tmp/install space"
mkdir -p "$proj/bin" "$tmp/tools"
proj="$(cd "$proj" && pwd -P)"
cp "$REPO_ROOT/bin/install" "$REPO_ROOT/bin/build" "$proj/bin/"
printf '#!/usr/bin/env bash\nprintf "checkout launcher\\n"\n' >"$proj/bin/jirafs"
chmod +x "$proj/bin/jirafs"
cp "$proj/bin/jirafs" "$tmp/original-launcher"
cat >"$tmp/tools/go" <<'GO'
#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
    if [[ "$1" == -o ]]; then output="$2"; break; fi
    shift
done
mkdir -p "$(dirname "$output")"
printf '#!/usr/bin/env bash\nprintf "binary snapshot\\n"\n' >"$output"
chmod +x "$output"
GO
chmod +x "$tmp/tools/go"
run_install() { PATH="$tmp/tools:$PATH" "$proj/bin/install" --prefix "$prefix" "$@"; }
run_install >/dev/null
[[ -f "$prefix/bin/jirafs" && ! -L "$prefix/bin/jirafs" ]] || fail 'default install is not a binary snapshot'
[[ "$("$prefix/bin/jirafs")" == 'binary snapshot' ]] || fail 'snapshot install did not execute'
pass 'default installation remains a binary snapshot'
status=0
run_install --link >/dev/null 2>"$tmp/error" || status=$?
[[ "$status" != 0 && ! -L "$prefix/bin/jirafs" ]] || fail 'link mode overwrote existing binary without force'
run_install --link --force >/dev/null
[[ "$(readlink "$prefix/bin/jirafs")" == "$proj/bin/jirafs" ]] || fail 'link did not target checkout launcher'
run_install --link >/dev/null
pass 'link mode requires force to replace a binary and is idempotent afterward'
printf 'untouched\n' >"$tmp/victim"
rm "$prefix/bin/jirafs"
ln -s "$tmp/victim" "$prefix/bin/jirafs"
status=0
run_install --link >/dev/null 2>/dev/null || status=$?
[[ "$status" != 0 ]] || fail 'conflicting symlink accepted without force'
run_install --link --force >/dev/null
[[ "$(cat "$tmp/victim")" == untouched ]] || fail 'conflicting symlink target was modified'
pass 'force replaces conflicting symlinks without modifying their targets'
status=0
run_install >/dev/null 2>/dev/null || status=$?
[[ "$status" != 0 ]] || fail 'snapshot mode accepted existing launcher symlink without force'
run_install --force >/dev/null
[[ ! -L "$prefix/bin/jirafs" ]] || fail 'snapshot mode did not replace symlink'
cmp "$tmp/original-launcher" "$proj/bin/jirafs" || fail 'snapshot install overwrote checkout launcher'
pass 'switching back to a snapshot preserves the checkout launcher'
# A symlink to a directory must be replaced, never used as the install directory.
rm "$prefix/bin/jirafs"
mkdir "$tmp/victim-dir"
ln -s "$tmp/victim-dir" "$prefix/bin/jirafs"
run_install --link --force >/dev/null
[[ "$(readlink "$prefix/bin/jirafs")" == "$proj/bin/jirafs" ]] || fail 'directory symlink was followed instead of replaced'
[[ -z "$(ls -A "$tmp/victim-dir")" ]] || fail 'directory symlink target received an install file'
pass 'a conflicting directory symlink is replaced without writing into its target'

status=0
PATH="$tmp/tools:$PATH" "$proj/bin/install" --prefix "$proj" --link --force >/dev/null 2>/dev/null || status=$?
[[ "$status" != 0 ]] || fail 'installer replaced its own tracked checkout launcher'
cmp "$tmp/original-launcher" "$proj/bin/jirafs" || fail 'installer modified its own launcher'
pass 'installer refuses to overwrite its own checkout launcher'
