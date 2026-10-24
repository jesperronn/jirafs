package cli

import (
	"os"
	"strings"
	"testing"
)

func TestStatusGit_UnpushedCommitsAndDirtyFiles(t *testing.T) {
	e := syncedEnv(t)
	t.Chdir(e.local)
	e.userCommit(t, "Summary TEST-1", "First local", "tweak summary")
	e.userCommit(t, "First local", "Second local", "tweak again")
	if err := os.WriteFile(e.file(), append(mustRead(t, e.file()), []byte("dirty line\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.mirror+"/.jirafs/scratch", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := withStatusTestIO(t)
	if exit := RunStatus([]string{"--project", "test"}); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr.String())
	}
	out := stdout.String()
	t.Logf("status output:\n%s", out)
	sha := shortSHA(strings.TrimSpace(gitOut(t, e.mirror, "log", "--grep", "^sync from", "-n", "1", "--format=%H")))
	for _, want := range []string{
		"Git:\n  2 local commits not pushed\n",
		"tweak again", "tweak summary",
		"  1 files with uncommitted edits\n",
		"inspect: git -C " + e.mirror + " log --oneline " + sha + "..HEAD\n",
		"run: jirafs push\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}

	stdout.Reset()
	if exit := RunStatus([]string{"--project", "test", "--porcelain"}); exit != 0 {
		t.Fatalf("porcelain exit %d", exit)
	}
	pl := stdout.String()
	t.Logf("porcelain output:\n%s", pl)
	var rows []string
	for _, l := range strings.Split(strings.TrimRight(pl, "\n"), "\n") {
		if strings.HasPrefix(l, "unpushed-commit\t") {
			if len(strings.Split(l, "\t")) != 3 {
				t.Errorf("row %q is not 3 columns", l)
			}
			rows = append(rows, l)
		}
	}
	if len(rows) != 2 || !strings.HasSuffix(rows[0], "\ttweak again") || !strings.HasSuffix(rows[1], "\ttweak summary") {
		t.Errorf("rows = %q", rows)
	}

	// A push commit makes the earlier commits pushed.
	gitOut(t, e.mirror, "commit", "-am", "wip")
	gitOut(t, e.mirror, "commit", "--allow-empty", "-m", "push to example.atlassian.net at 2026-01-03T00:00:00Z")
	stdout.Reset()
	if exit := RunStatus([]string{"--project", "test"}); exit != 0 {
		t.Fatalf("exit %d", exit)
	}
	out = stdout.String()
	if !strings.Contains(out, "0 local commits not pushed") || !strings.Contains(out, "0 files with uncommitted edits") || strings.Contains(out, "run: jirafs push\n") {
		t.Errorf("after push commit:\n%s", out)
	}
	stdout.Reset()
	RunStatus([]string{"--project", "test", "--porcelain"})
	if strings.Contains(stdout.String(), "unpushed-commit") {
		t.Errorf("porcelain still lists unpushed commits:\n%s", stdout.String())
	}
}

func TestStatusGit_OffAndNoSyncCommit(t *testing.T) {
	e := newGitPullEnv(t, false)
	t.Chdir(e.local)
	stdout, _ := withStatusTestIO(t)
	RunStatus([]string{"--project", "test"})
	if strings.Contains(stdout.String(), "Git:") {
		t.Errorf("git section shown with git mode off:\n%s", stdout.String())
	}

	e = newGitPullEnv(t, true)
	t.Chdir(e.local)
	e.initRepo(t)
	if err := os.WriteFile(e.mirror+"/notes.txt", []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _ = withStatusTestIO(t)
	RunStatus([]string{"--project", "test"})
	out := stdout.String()
	if !strings.Contains(out, "0 local commits not pushed") || !strings.Contains(out, "1 files with uncommitted edits") || !strings.Contains(out, "inspect: git -C "+e.mirror+" status") {
		t.Errorf("output:\n%s", out)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
