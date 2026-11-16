#!/usr/bin/env bash
# Check cached launches, symlink resolution and source invalidation without Jira.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
pass() { echo "[PASS] $1"; }
fail() { echo "[FAIL] $1" >&2; exit 1; }
has() { case "$1" in *"$2"*) ;; *) fail "expected '$2' in: $1" ;; esac; }

[[ -x "$REPO_ROOT/bin/jirafs" ]] || fail "checkout launcher does not exist"
proj="$tmp/project space"
mkdir -p "$proj/bin" "$tmp/tools" "$tmp/elsewhere" "$tmp/links"
cp "$REPO_ROOT/bin/jirafs" "$REPO_ROOT/bin/build" "$proj/bin/"
cat >"$tmp/tools/go" <<'GO'
#!/usr/bin/env bash
printf 'build\n' >>"$BUILD_LOG"
printf 'build stdout belongs on stderr\n'
[[ "${FAIL_BUILD:-0}" == 0 ]] || exit 7
while [[ $# -gt 0 ]]; do
    if [[ "$1" == -o ]]; then output="$2"; break; fi
    shift
done
cat >"$output" <<'APP'
#!/usr/bin/env bash
printf 'cwd:%s\n' "$PWD"
printf 'arg:%s\n' "$@"
exit "${APP_EXIT:-0}"
APP
chmod +x "$output"
GO
chmod +x "$tmp/tools/go"
export BUILD_LOG="$tmp/builds"
# Exercise both absolute and relative symlink targets from a different directory.
ln -s "$proj/bin/jirafs" "$tmp/links/target"
ln -s target "$tmp/links/jirafs"
cd "$tmp/elsewhere"
output="$(PATH="$tmp/tools:$PATH" "$tmp/links/jirafs" 'two words' '' '*.go' 2>"$tmp/stderr")"
has "$output" "cwd:$tmp/elsewhere"
has "$output" $'arg:two words\narg:\narg:*.go'
if [[ "$output" == *'build stdout'* ]]; then fail 'build output polluted application stdout'; fi
has "$(cat "$tmp/stderr")" 'build stdout belongs on stderr'
pass 'symlinked launcher preserves caller directory, arguments and clean stdout'

PATH="$tmp/tools:$PATH" "$proj/bin/jirafs" --help >/dev/null 2>/dev/null
[[ "$(wc -l <"$BUILD_LOG" | tr -d ' ')" == 2 ]] || fail 'launcher skipped the second Go build'
pass 'every invocation delegates freshness to bin/build and the Go cache'

status=0
PATH="$tmp/tools:$PATH" APP_EXIT=23 "$proj/bin/jirafs" >/dev/null 2>/dev/null || status=$?
[[ "$status" == 23 ]] || fail "application exit status changed: $status"
pass 'launcher preserves application exit status'

cp "$proj/.bin/jirafs" "$tmp/previous-binary"
status=0
PATH="$tmp/tools:$PATH" FAIL_BUILD=1 "$proj/bin/jirafs" >"$tmp/failed-out" 2>"$tmp/failed-err" || status=$?
[[ "$status" == 7 ]] || fail "build exit status changed: $status"
[[ ! -s "$tmp/failed-out" ]] || fail 'a failed build ran the previous binary'
cmp "$tmp/previous-binary" "$proj/.bin/jirafs" || fail 'failed build replaced the cached binary'
[[ -z "$(find "$proj/.bin" -name '.jirafs.*' -print)" ]] || fail 'temporary binary leaked'
pass 'build failure never executes stale output and preserves the old binary atomically'

# Real Go validates edits, added/deleted files and go:embed cache invalidation.
mkdir -p "$proj/cmd/jirafs"
printf 'module example.test/launcher\n\ngo 1.26\n' >"$proj/go.mod"
printf 'embed-one' >"$proj/cmd/jirafs/message.txt"
printf 'package main\nvar marker = "source-one"\n' >"$proj/cmd/jirafs/state.go"
cat >"$proj/cmd/jirafs/main.go" <<'GO'
package main
import (
    _ "embed"
    "fmt"
)
//go:embed message.txt
var message string
var version string
func main() { fmt.Printf("%s|%s", marker, message) }
GO
run_real() { GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local "$proj/bin/jirafs" 2>"$tmp/real-stderr"; }
[[ "$(run_real)" == 'source-one|embed-one' ]] || fail 'initial Go build did not execute current source'
printf 'package main\nvar marker = "source-two"\n' >"$proj/cmd/jirafs/state.go"
[[ "$(run_real)" == 'source-two|embed-one' ]] || fail 'source edit did not invalidate cache'
printf 'package main\nfunc init() { marker = "added" }\n' >"$proj/cmd/jirafs/extra.go"
[[ "$(run_real)" == 'added|embed-one' ]] || fail 'added source did not invalidate cache'
rm "$proj/cmd/jirafs/extra.go"
[[ "$(run_real)" == 'source-two|embed-one' ]] || fail 'deleted source did not invalidate cache'
printf 'embed-two' >"$proj/cmd/jirafs/message.txt"
[[ "$(run_real)" == 'source-two|embed-two' ]] || fail 'embedded-file edit did not invalidate cache'
rm "$proj/.bin/jirafs"
[[ "$(run_real)" == 'source-two|embed-two' ]] || fail 'warm cache did not restore missing binary'
pass 'Go cache observes source edits, added/deleted sources, embeds and missing binary'
