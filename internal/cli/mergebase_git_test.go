package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/gitx"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

// summaryOf returns the base summary or "<none>".
func summaryOf(i *schema.Issue, ok bool) string {
	if !ok {
		return "<none>"
	}
	return i.Summary
}

func (e *gitPullEnv) file() string { return filepath.Join(e.local, "TEST-1.md") }

func (e *gitPullEnv) userCommit(t *testing.T, old, repl, msg string) {
	t.Helper()
	data, err := os.ReadFile(e.file())
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Replace(string(data), old, repl, 1)
	if out == string(data) {
		t.Fatalf("%q not in file:\n%s", old, data)
	}
	if err := os.WriteFile(e.file(), []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, e.mirror, "commit", "-am", msg)
}

// syncedEnv has a git mirror with TEST-1 pulled and committed by jirafs.
func syncedEnv(t *testing.T) *gitPullEnv {
	t.Helper()
	e := newGitPullEnv(t, true)
	e.initRepo(t)
	e.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1")
	if code, out := e.pull(t, "--full", "my-issues"); code != 0 {
		t.Fatalf("pull = %d: %s", code, out)
	}
	return e
}

func poisonStoredBase(t *testing.T, e *gitPullEnv) {
	t.Helper()
	if err := mirror.SaveBase(e.mirror, &schema.Issue{
		Identity: schema.IssueIdentity{Key: "TEST-1", Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}},
		Summary:  "STORED",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMergeBase_FromSyncCommitEqualsStoredBase(t *testing.T) {
	e := syncedEnv(t)
	stored, ok := mirror.LoadBase(e.mirror, "TEST-1")
	if !ok {
		t.Fatal("no stored base")
	}
	got, ok := loadMergeBase(e.mirror, "TEST-1", e.file())
	if !ok || got.Summary != stored.Summary || got.Summary != "Summary TEST-1 2026-01-01T00:00:00.000+0000" {
		t.Fatalf("git base = %q, stored = %q", summaryOf(got, ok), stored.Summary)
	}
}

func TestLoadMergeBase_InterleavedUserCommitDoesNotMoveBase(t *testing.T) {
	e := syncedEnv(t)
	poisonStoredBase(t, e)
	e.userCommit(t, "Summary TEST-1", "User title", "my edit")
	got, ok := loadMergeBase(e.mirror, "TEST-1", e.file())
	t.Logf("base summary taken from git (stored base is STORED, HEAD is a user commit): %q", summaryOf(got, ok))
	if got.Summary != "Summary TEST-1 2026-01-01T00:00:00.000+0000" {
		t.Fatalf("base = %q, want the sync commit's version", got.Summary)
	}
}

func TestLoadMergeBase_PushCommitBecomesBase(t *testing.T) {
	e := syncedEnv(t)
	e.userCommit(t, "Summary TEST-1", "Pushed title", "my edit")
	gitOut(t, e.mirror, "commit", "--allow-empty", "-m", "push to example.atlassian.net at 2026-01-03T00:00:00Z")
	got, ok := loadMergeBase(e.mirror, "TEST-1", e.file())
	if !strings.HasPrefix(summaryOf(got, ok), "Pushed title") {
		t.Fatalf("base = %q, want the user commit before the push commit", summaryOf(got, ok))
	}
}

func TestLoadMergeBase_Fallbacks(t *testing.T) {
	t.Run("rewritten history", func(t *testing.T) {
		e := syncedEnv(t)
		poisonStoredBase(t, e)
		gitOut(t, e.mirror, "commit", "--amend", "-m", "rewritten")
		got, ok := loadMergeBase(e.mirror, "TEST-1", e.file())
		if summaryOf(got, ok) != "STORED" {
			t.Fatalf("base = %q, want stored fallback", summaryOf(got, ok))
		}
	})
	t.Run("file added after sync commit", func(t *testing.T) {
		e := syncedEnv(t)
		p := filepath.Join(e.local, "TEST-9.md")
		if err := os.WriteFile(p, []byte("---\nkey: 'TEST-9'\ntype: 'story'\nproject: 'project:TEST'\nsummary: 'x'\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitOut(t, e.mirror, "add", "-A")
		gitOut(t, e.mirror, "commit", "-m", "add 9")
		if got, ok := loadMergeBase(e.mirror, "TEST-9", p); ok {
			t.Fatalf("base = %q, want none", got.Summary)
		}
	})
	t.Run("git mode off", func(t *testing.T) {
		e := newGitPullEnv(t, false)
		e.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1")
		if code, out := e.pull(t, "--full", "my-issues"); code != 0 {
			t.Fatalf("pull = %d: %s", code, out)
		}
		got, ok := loadMergeBase(e.mirror, "TEST-1", e.file())
		if !ok || !strings.HasPrefix(got.Summary, "Summary TEST-1") {
			t.Fatalf("base = %q", summaryOf(got, ok))
		}
	})
	t.Run("not a repository", func(t *testing.T) {
		e := newGitPullEnv(t, true)
		poisonStoredBase(t, e)
		got, ok := loadMergeBase(e.mirror, "TEST-1", e.file())
		if summaryOf(got, ok) != "STORED" {
			t.Fatalf("base = %q", summaryOf(got, ok))
		}
	})
	t.Run("repository without commits", func(t *testing.T) {
		e := newGitPullEnv(t, true)
		gitOut(t, e.mirror, "init", "-b", "main")
		if got, ok := loadMergeBase(e.mirror, "TEST-1", e.file()); ok {
			t.Fatalf("base = %q, want none", got.Summary)
		}
	})
	t.Run("no sync commit yet", func(t *testing.T) {
		e := newGitPullEnv(t, true)
		e.initRepo(t)
		if _, ok := loadMergeBase(e.mirror, "TEST-1", e.file()); ok {
			t.Fatal("want no base")
		}
	})
	t.Run("unparseable content at the commit", func(t *testing.T) {
		e := newGitPullEnv(t, true)
		poisonStoredBase(t, e)
		if err := os.WriteFile(e.file(), []byte("not an issue\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		e.initRepo(t)
		gitOut(t, e.mirror, "commit", "--allow-empty", "-m", "sync from example.atlassian.net at 2026-01-01T00:00:00Z")
		got, ok := loadMergeBase(e.mirror, "TEST-1", e.file())
		if summaryOf(got, ok) != "STORED" {
			t.Fatalf("base = %q", summaryOf(got, ok))
		}
	})
	t.Run("invalid mirror.yml and empty args", func(t *testing.T) {
		if _, ok := loadMergeBase("", "TEST-1", "x"); ok {
			t.Fatal("empty mirror dir")
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "mirror.yml"), []byte(":\n\t bad"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, ok := loadMergeBase(dir, "TEST-1", filepath.Join(dir, "a.md")); ok {
			t.Fatal("invalid mirror.yml")
		}
	})
}

func TestLoadMergeBase_DetachedHeadAndShallowClone(t *testing.T) {
	e := syncedEnv(t)
	gitOut(t, e.mirror, "checkout", "--detach")
	got, ok := loadMergeBase(e.mirror, "TEST-1", e.file())
	if !ok || !strings.HasPrefix(got.Summary, "Summary TEST-1") {
		t.Fatalf("detached: base = %q", summaryOf(got, ok))
	}
	e.userCommit(t, "Summary TEST-1", "User title", "my edit")
	clone := filepath.Join(t.TempDir(), "clone")
	gitOut(t, e.root, "clone", "--depth", "1", "file://"+e.mirror, clone)
	if err := os.WriteFile(filepath.Join(clone, "mirror.yml"), []byte("project:\n  type: project\n  value: TEST\ngit: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	poison := &schema.Issue{Identity: schema.IssueIdentity{Key: "TEST-1"}, Summary: "STORED"}
	if err := mirror.SaveBase(clone, poison); err != nil {
		t.Fatal(err)
	}
	// The shallow history holds only the user commit: no sync commit is
	// visible, so the stored base is used.
	got, ok = loadMergeBase(clone, "TEST-1", filepath.Join(clone, "issues", "TEST-1.md"))
	if summaryOf(got, ok) != "STORED" {
		t.Fatalf("shallow: base = %q", summaryOf(got, ok))
	}
}

func TestPullGit_ConflictUsesSyncCommitBaseDespiteUserCommit(t *testing.T) {
	e := syncedEnv(t)
	poisonStoredBase(t, e)
	e.userCommit(t, "Summary TEST-1", "User title", "my edit")
	e.setIssues("2026-01-02T00:00:00.000+0000", "TEST-1")
	if code, out := e.pull(t, "--full", "my-issues"); code != 0 {
		t.Fatalf("pull = %d: %s", code, out)
	}
	data, err := os.ReadFile(e.file())
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	t.Logf("conflicted file after pull with an interleaved user commit:\n%s", got)
	for _, want := range []string{"summary: 'User title 2026-01-01", "conflicts:", "field: summary", "base: Summary TEST-1 2026-01-01T00:00:00.000+0000", "remote: Summary TEST-1 2026-01-02T00:00:00.000+0000"} {
		if !strings.Contains(got, want) {
			t.Errorf("file lacks %q", want)
		}
	}
	if strings.Contains(got, "STORED") {
		t.Error("stored base was used in git mode")
	}
}

func TestGitxSyncHelpers(t *testing.T) {
	gitx.RequireGit(t)
	gitx.HermeticEnv(t)
	dir := t.TempDir()
	if err := gitx.Init(dir); err != nil {
		t.Fatal(err)
	}
	if sha, err := gitx.LastSyncCommit(dir); err != nil || sha != "" {
		t.Fatalf("empty repo: %q, %v", sha, err)
	}
	if _, err := gitx.LastSyncCommit(t.TempDir()); err == nil {
		// a non-repository has no HEAD: treated as no commits
		t.Log("non-repo yields no commit")
	}
	if _, ok := gitx.ShowFile(dir, "deadbeef", "a.md"); ok {
		t.Fatal("ShowFile of a missing commit succeeded")
	}
	if _, err := gitx.CommitsSince(dir, "deadbeef", ""); err == nil {
		t.Fatal("CommitsSince with a bad commit should fail")
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-m", "sync from h at t")
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "commit", "-am", "later")
	sha, err := gitx.LastSyncCommit(dir)
	if err != nil || sha == "" {
		t.Fatalf("sha = %q, %v", sha, err)
	}
	if c, ok := gitx.ShowFile(dir, sha, "a.md"); !ok || c != "one\n" {
		t.Fatalf("ShowFile = %q, %v", c, ok)
	}
	cs, err := gitx.CommitsSince(dir, sha, ".")
	if err != nil || len(cs) != 1 || cs[0].Subject != "later" {
		t.Fatalf("CommitsSince = %+v, %v", cs, err)
	}
}
