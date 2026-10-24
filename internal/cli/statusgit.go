package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/jirafs/jirafs/internal/gitx"
)

// GitStatus is the git-mode part of a status report: what happened locally
// since the last `sync from` / `push to` commit. It is built offline.
type GitStatus struct {
	// Dir is the mirror directory (the repository work tree used).
	Dir string
	// SyncCommit is the full hash of the last sync/push commit, "" if none.
	SyncCommit string
	// Unpushed lists commits after SyncCommit touching the mirror, newest first.
	Unpushed []gitx.LogEntry
	// Dirty counts mirror files with uncommitted edits (jirafs state excluded).
	Dirty int
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// buildGitStatus returns nil unless mirrorDir has `git: true`, git is usable
// and the directory is a repository.
func buildGitStatus(mirrorDir string) *GitStatus {
	if mirrorDir == "" || !gitx.Available() {
		return nil
	}
	m, _, err := loadMirrorYAML(mirrorDir)
	if err != nil || !m.Git || !gitx.IsRepo(mirrorDir) {
		return nil
	}
	gs := &GitStatus{Dir: mirrorDir}
	if sha, err := gitx.LastSyncCommit(mirrorDir); err == nil {
		gs.SyncCommit = sha
	}
	if gs.SyncCommit != "" {
		if commits, err := gitx.CommitsSince(mirrorDir, gs.SyncCommit, "."); err == nil {
			gs.Unpushed = commits
		}
	}
	if entries, err := gitx.Status(mirrorDir, "."); err == nil {
		for _, e := range entries {
			if e.Path == ".jirafs" || strings.HasPrefix(e.Path, ".jirafs/") || strings.Contains(e.Path, "/.jirafs/") {
				continue
			}
			gs.Dirty++
		}
	}
	return gs
}

// printPorcelain writes the unpushed-commit rows.
func (g *GitStatus) printPorcelain(w io.Writer) {
	if g == nil {
		return
	}
	for _, c := range g.Unpushed {
		fmt.Fprintf(w, "unpushed-commit\t%s\t%s\n", shortSHA(c.Hash), c.Subject)
	}
}

// print writes the "Git:" section of the human status output.
func (g *GitStatus) print(w io.Writer) {
	if g == nil {
		return
	}
	fmt.Fprintln(w, "Git:")
	fmt.Fprintf(w, "  %d local commits not pushed\n", len(g.Unpushed))
	for _, c := range g.Unpushed {
		fmt.Fprintf(w, "    %s  %s\n", shortSHA(c.Hash), c.Subject)
	}
	fmt.Fprintf(w, "  %d files with uncommitted edits\n", g.Dirty)
	if len(g.Unpushed) > 0 || g.Dirty > 0 {
		if g.SyncCommit != "" {
			fmt.Fprintf(w, "    inspect: git -C %s log --oneline %s..HEAD\n", g.Dir, shortSHA(g.SyncCommit))
		} else {
			fmt.Fprintf(w, "    inspect: git -C %s status\n", g.Dir)
		}
		fmt.Fprintln(w, "    run: jirafs push")
	}
	fmt.Fprintln(w)
}
