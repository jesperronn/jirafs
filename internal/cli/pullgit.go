package cli

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jirafs/jirafs/internal/gitx"
	"github.com/jirafs/jirafs/internal/schema"
)

// pullGit tracks one pull in a git-backed mirror so the files the pull wrote
// can be committed afterwards. A nil *pullGit means git mode is off and every
// method is a no-op.
type pullGit struct {
	mirrorDir string
	dirty     map[string]bool // absolute paths with uncommitted changes before the pull
}

// pullGitCommit describes what a pull wrote and what to say about it.
type pullGitCommit struct {
	// Label is the scope name or issue key the pull was run for.
	Label string
	// Host is the Jira host shown in the commit subject.
	Host string
	// Written maps each written file to its issue key.
	Written map[string]schema.IssueKey
	// Archived and Moved list issue keys archived or moved by the pull.
	Archived []schema.IssueKey
	Moved    []schema.IssueKey
	// Origin maps an archived file to the live path it was moved from; both
	// are in Written, and the archive is left uncommitted when the live file
	// had uncommitted edits before the pull.
	Origin map[string]string
	// Scope is true when Label names a scope rather than an issue key.
	Scope bool
}

// beginPullGit returns a tracker when mirrorDir is git-backed (`git: true`),
// the pull is not a dry run, and git is usable; it snapshots which files have
// uncommitted changes before the pull overwrites anything.
func beginPullGit(mirrorDir string, dryRun bool, stderr io.Writer) *pullGit {
	if dryRun || mirrorDir == "" {
		return nil
	}
	m, _, err := loadMirrorYAML(mirrorDir)
	if err != nil || !m.Git {
		return nil
	}
	g := &pullGit{mirrorDir: realPath(mirrorDir), dirty: map[string]bool{}}
	if !gitx.Available() || !gitx.IsRepo(g.mirrorDir) {
		return g
	}
	top, err := gitx.Toplevel(g.mirrorDir)
	if err != nil {
		return g
	}
	entries, err := gitx.Status(g.mirrorDir)
	if err != nil {
		return g
	}
	top = realPath(top)
	for _, e := range entries {
		g.dirty[filepath.Join(top, filepath.FromSlash(e.Path))] = true
	}
	return g
}

func realPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	// The file may not exist yet; resolve its directory instead.
	if r, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		return filepath.Join(r, filepath.Base(p))
	}
	return p
}

// within reports whether path is inside dir.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// gitBlocker reports why committing is unsafe right now, or "".
func gitBlocker(dir string) string {
	if _, err := gitx.Run(dir, "symbolic-ref", "-q", "HEAD"); err != nil {
		// A repository without commits has an unborn branch, which still
		// resolves; failure here means a detached HEAD.
		return "detached HEAD; not committing (run: git -C " + dir + " switch main)"
	}
	for _, name := range []string{"rebase-merge", "rebase-apply", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD"} {
		if p := gitPath(dir, name); p != "" && exists(p) {
			return "a rebase/merge is in progress; not committing (finish it, then run: git -C " + dir + " status)"
		}
	}
	if p := gitPath(dir, "index.lock"); p != "" && exists(p) {
		return "git index is locked; not committing (if no git process is running: rm " + p + ")"
	}
	return ""
}

func gitPath(dir, name string) string {
	out, err := gitx.Run(dir, "rev-parse", "--git-path", name)
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(out)
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	return p
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// commit stages exactly the written files that had no uncommitted edits
// before the pull and commits them. It never fails the pull: problems are
// reported on stderr.
func (g *pullGit) commit(c pullGitCommit, now time.Time, stdout, stderr io.Writer) {
	if g == nil || len(c.Written) == 0 {
		return
	}
	if !gitx.Available() {
		fmt.Fprintf(stderr, "jirafs pull: git mode is on but git is missing; not committing (%s)\n", gitx.NotInstalledHint)
		return
	}
	if !gitx.IsRepo(g.mirrorDir) {
		fmt.Fprintf(stderr, "jirafs pull: mirror has git: true but %s is not a git repository; not committing (run: git -C %s init)\n", g.mirrorDir, g.mirrorDir)
		return
	}
	if msg := gitBlocker(g.mirrorDir); msg != "" {
		fmt.Fprintf(stderr, "jirafs pull: %s\n", msg)
		return
	}

	var paths []string
	var changed []string
	var left []string
	for path, key := range c.Written {
		abs := realPath(path)
		if !within(g.mirrorDir, abs) {
			continue
		}
		if g.dirty[abs] || (c.Origin[path] != "" && g.dirty[realPath(c.Origin[path])]) {
			left = append(left, string(key))
			continue
		}
		paths = append(paths, abs)
		if !containsStr(changed, string(key)) {
			changed = append(changed, string(key))
		}
	}
	sort.Strings(paths)
	sort.Strings(changed)
	sort.Strings(left)
	left = dedupeSorted(left)
	for _, key := range left {
		fmt.Fprintf(stderr, "left uncommitted: %s (local edits)\n", key)
	}
	if len(paths) == 0 {
		return
	}
	entries, err := gitx.Status(g.mirrorDir, paths...)
	if err != nil {
		g.fail(err, stderr)
		return
	}
	if len(entries) == 0 {
		return
	}

	if err := gitx.Commit(g.mirrorDir, paths, pullCommitMessage(c, changed, now)); err != nil {
		if errors.Is(err, gitx.ErrNoIdentity) {
			fmt.Fprintf(stderr, "pull ok, git commit failed: %v\n", err)
			fmt.Fprintf(stderr, "then commit: git -C %s status\n", g.mirrorDir)
			return
		}
		g.fail(err, stderr)
		return
	}
	fmt.Fprintf(stdout, "jirafs pull: committed %d file(s) to git\n", len(paths))
}

func (g *pullGit) fail(err error, stderr io.Writer) {
	fmt.Fprintf(stderr, "pull ok, git commit failed: %v\n", err)
	fmt.Fprintf(stderr, "inspect: git -C %s status\n", g.mirrorDir)
}

func pullCommitMessage(c pullGitCommit, changed []string, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "sync from %s at %s\n\n", c.Host, now.UTC().Format(time.RFC3339))
	if c.Scope {
		fmt.Fprintf(&b, "scope: %s\n", c.Label)
	} else {
		fmt.Fprintf(&b, "issue: %s\n", c.Label)
	}
	fmt.Fprintf(&b, "changed: %s\n", strings.Join(changed, ", "))
	if len(c.Archived) > 0 {
		fmt.Fprintf(&b, "archived: %s\n", joinKeys(c.Archived))
	}
	if len(c.Moved) > 0 {
		fmt.Fprintf(&b, "moved: %s\n", joinKeys(c.Moved))
	}
	return b.String()
}

func joinKeys(keys []schema.IssueKey) string {
	s := make([]string, len(keys))
	for i, k := range keys {
		s[i] = string(k)
	}
	sort.Strings(s)
	return strings.Join(s, ", ")
}

// hostOf returns the host of a Jira base URL, or the URL itself when it does
// not parse.
func hostOf(baseURL string) string {
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		return u.Host
	}
	return baseURL
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func dedupeSorted(in []string) []string {
	out := in[:0]
	for i, v := range in {
		if i == 0 || v != in[i-1] {
			out = append(out, v)
		}
	}
	return out
}
