// Package gitx is a thin wrapper around the git binary. It shells out to
// git and parses nothing except porcelain v1 -z status output.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// NotInstalledHint is the message shown when git is missing.
const NotInstalledHint = "jirafs: git is not installed or not on PATH; install it (brew install git / apt install git) or run: jirafs init"

// IdentityHint is the message shown when git has no committer identity.
const IdentityHint = "git has no user identity; run: git config --global user.name \"Your Name\" && git config --global user.email you@example.com"

// ErrNotInstalled is returned when the git binary cannot be found.
var ErrNotInstalled = errors.New(NotInstalledHint)

// ErrNoIdentity is returned when a commit fails for lack of user.name/user.email.
var ErrNoIdentity = errors.New(IdentityHint)

// Available reports whether a git binary is on PATH.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// Run executes git with args in dir and returns its stdout. On failure the
// error includes git's stderr.
func Run(dir string, args ...string) (string, error) {
	if !Available() {
		return "", ErrNotInstalled
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// IsRepo reports whether dir is inside a git work tree (its own or a parent's).
func IsRepo(dir string) bool {
	out, err := Run(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// Toplevel returns the root of the work tree containing dir.
func Toplevel(dir string) (string, error) {
	out, err := Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Init runs git init in dir.
func Init(dir string) error {
	_, err := Run(dir, "init")
	return err
}

// StatusEntry is one porcelain v1 status record.
type StatusEntry struct {
	// XY is the two-letter status code.
	XY string
	// Path is the path relative to the repository root.
	Path string
}

// Status returns the porcelain v1 -z status of paths (all paths when empty).
func Status(dir string, paths ...string) ([]StatusEntry, error) {
	args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	out, err := Run(dir, args...)
	if err != nil {
		return nil, err
	}
	var entries []StatusEntry
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		e := StatusEntry{XY: f[:2], Path: f[3:]}
		if f[0] == 'R' || f[0] == 'C' {
			i++ // the original path follows as a separate field
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// Commit stages paths and commits them with msg. It returns ErrNoIdentity
// when git lacks user.name/user.email.
func Commit(dir string, paths []string, msg string) error {
	addArgs := append([]string{"add", "-A", "--"}, paths...)
	if _, err := Run(dir, addArgs...); err != nil {
		return err
	}
	commitArgs := append([]string{"commit", "-m", msg, "--"}, paths...)
	if _, err := Run(dir, commitArgs...); err != nil {
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "user.name") || strings.Contains(low, "user.email") ||
			strings.Contains(low, "author identity unknown") || strings.Contains(low, "empty ident name") {
			return ErrNoIdentity
		}
		return err
	}
	return nil
}

// SyncSubjectPattern matches the subjects of the commits jirafs makes after a
// pull (`sync from <host> ...`) or a push (`push to <host> ...`).
const SyncSubjectPattern = `^\(sync from\|push to\) `

// LastSyncCommit returns the full hash of the newest commit on the current
// branch whose subject starts with `sync from ` or `push to `, or "" when
// there is none (including a repository without commits). Reading is safe on
// a detached HEAD and in a shallow clone.
func LastSyncCommit(dir string) (string, error) {
	if _, err := Run(dir, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		return "", nil // unborn branch: no commits yet
	}
	out, err := Run(dir, "log", "--grep", SyncSubjectPattern, "-n", "1", "--format=%H")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ShowFile returns the content of the file at path (absolute, or relative to
// dir) as of commit. ok is false when the commit or the file does not exist
// there, for instance after history was rewritten or for a file added later.
func ShowFile(dir, commit, path string) (content string, ok bool) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	fileDir := filepath.Dir(path)
	out, err := Run(fileDir, "show", commit+":./"+filepath.Base(path))
	if err != nil {
		return "", false
	}
	return out, true
}

// CommitsSince returns the commits reachable from HEAD but not from commit that touch pathspec (the
// whole tree when empty), newest first.
func CommitsSince(dir, commit, pathspec string) ([]LogEntry, error) {
	args := []string{"log", "--format=%H%x00%s", commit + "..HEAD"}
	if pathspec != "" {
		args = append(args, "--", pathspec)
	}
	out, err := Run(dir, args...)
	if err != nil {
		return nil, err
	}
	var commits []LogEntry
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		hash, subject, _ := strings.Cut(line, "\x00")
		commits = append(commits, LogEntry{Hash: hash, Subject: subject})
	}
	return commits, nil
}

// LogEntry identifies one commit in a log.
type LogEntry struct {
	Hash    string
	Subject string
}
