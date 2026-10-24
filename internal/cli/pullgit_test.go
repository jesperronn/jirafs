package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/gitx"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

type gitPullEnv struct {
	root, mirror, local string
	fake                *jira.FakeTransport
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitx.Run(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

func newGitPullEnv(t *testing.T, gitOn bool) *gitPullEnv {
	t.Helper()
	gitx.RequireGit(t)
	gitx.HermeticEnv(t)
	root := t.TempDir()
	home := filepath.Join(root, "home")
	mirror := filepath.Join(root, "mirror")
	local := filepath.Join(mirror, "issues")
	for _, d := range []string{filepath.Join(home, ".jirafs"), local} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	settings := "version = 1\n\n[instances.default]\nbase_url = \"https://example.atlassian.net\"\nauth_type = \"basic\"\n\n[projects.test]\nkey = \"TEST\"\ninstance = \"default\"\nmirror_dir = \"" + mirror + "\"\nlocal_dirs = [\"" + local + "\"]\n"
	if err := os.WriteFile(filepath.Join(home, ".jirafs", "settings.toml"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	yml := "project:\n  type: project\n  value: TEST\nscopes:\n  - name: my-issues\n    type: jql\n    target: assignee = currentUser()\n"
	if gitOn {
		yml += "git: true\n"
	}
	if err := os.WriteFile(filepath.Join(mirror, "mirror.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	fake := jira.NewFakeTransport()
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return fake, nil })
	withPullTestIO(t)
	return &gitPullEnv{root: root, mirror: mirror, local: local, fake: fake}
}

func (e *gitPullEnv) initRepo(t *testing.T) {
	t.Helper()
	gitOut(t, e.mirror, "init", "-b", "main")
	gitOut(t, e.mirror, "add", "-A")
	gitOut(t, e.mirror, "commit", "-m", "init")
}

func (e *gitPullEnv) setIssues(updated string, keys ...string) {
	var list []*schema.Issue
	for _, k := range keys {
		is := &schema.Issue{
			Identity:     schema.IssueIdentity{Key: schema.IssueKey(k), Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}},
			MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
			Summary:      "Summary " + k + " " + updated,
			Context:      schema.IssueContext{Updated: updated},
		}
		list = append(list, is)
		e.fake.SetIssue(k, is)
	}
	e.fake.SetIssuesByScope("my-issues", list)
}

func (e *gitPullEnv) pull(t *testing.T, args ...string) (int, string) {
	t.Helper()
	stdout, stderr := withPullTestIO(t)
	code := RunPull(append([]string{"--project", "test"}, args...))
	return code, stdout.String() + stderr.String()
}

func commitCount(t *testing.T, dir string) int {
	t.Helper()
	return len(strings.Fields(gitOut(t, dir, "rev-list", "--all")))
}

func TestPullGit_CommitsWrittenFiles(t *testing.T) {
	e := newGitPullEnv(t, true)
	e.initRepo(t)
	e.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1", "TEST-2")
	if code, out := e.pull(t, "--full", "my-issues"); code != 0 {
		t.Fatalf("pull = %d: %s", code, out)
	}
	subj := gitOut(t, e.mirror, "log", "-1", "--format=%s")
	if !regexp.MustCompile(`^sync from example\.atlassian\.net at \d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ\n$`).MatchString(subj) {
		t.Fatalf("subject = %q", subj)
	}
	body := gitOut(t, e.mirror, "log", "-1", "--format=%b")
	for _, want := range []string{"scope: my-issues", "changed: TEST-1, TEST-2"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body = %q, want %q", body, want)
		}
	}
	files := gitOut(t, e.mirror, "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(files, "issues/TEST-1.md") || !strings.Contains(files, "issues/TEST-2.md") || strings.Contains(files, "mirror.yml") {
		t.Fatalf("files = %q", files)
	}
	t.Log(gitOut(t, e.mirror, "log", "-2", "--stat"))

	// A second pull with nothing changed makes no commit.
	n := commitCount(t, e.mirror)
	if code, out := e.pull(t, "--full", "my-issues"); code != 0 {
		t.Fatalf("pull2 = %d: %s", code, out)
	}
	if commitCount(t, e.mirror) != n {
		t.Fatal("no-op pull created a commit")
	}
}

func TestPullGit_UserEditExcludedAndUnrelatedUntouched(t *testing.T) {
	e := newGitPullEnv(t, true)
	if err := os.WriteFile(filepath.Join(e.local, "TEST-1.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.mirror, "notes.txt"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.initRepo(t)
	// Uncommitted user edit to TEST-1 and an unrelated dirty file.
	if err := os.WriteFile(filepath.Join(e.local, "TEST-1.md"), []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.mirror, "notes.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1", "TEST-2")
	code, out := e.pull(t, "--full", "my-issues")
	if code != 0 {
		t.Fatalf("pull = %d: %s", code, out)
	}
	if !strings.Contains(out, "left uncommitted: TEST-1 (local edits)") {
		t.Fatalf("output = %q", out)
	}
	files := gitOut(t, e.mirror, "show", "--name-only", "--format=", "HEAD")
	if strings.Contains(files, "TEST-1.md") || !strings.Contains(files, "TEST-2.md") {
		t.Fatalf("commit files = %q", files)
	}
	if _, err := os.ReadFile(filepath.Join(e.local, "TEST-1.md")); err != nil {
		t.Fatalf("TEST-1.md missing: %v", err)
	}
	st := gitOut(t, e.mirror, "status", "--porcelain")
	if !strings.Contains(st, " M notes.txt") || !strings.Contains(st, " M issues/TEST-1.md") {
		t.Fatalf("status = %q", st)
	}
}

func TestPullGit_DetachedHeadSkips(t *testing.T) {
	e := newGitPullEnv(t, true)
	e.initRepo(t)
	gitOut(t, e.mirror, "checkout", "--detach")
	n := commitCount(t, e.mirror)
	e.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1")
	code, out := e.pull(t, "--full", "my-issues")
	if code != 0 || !strings.Contains(out, "detached HEAD") || !strings.Contains(out, "switch main") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if commitCount(t, e.mirror) != n {
		t.Fatal("committed on detached HEAD")
	}
	if _, err := os.Stat(filepath.Join(e.local, "TEST-1.md")); err != nil {
		t.Fatal("file not written")
	}
}

func TestPullGit_InProgressAndLocked(t *testing.T) {
	e := newGitPullEnv(t, true)
	e.initRepo(t)
	gitDir := filepath.Join(e.mirror, ".git")
	if err := os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1")
	if code, out := e.pull(t, "--full", "my-issues"); code != 0 || !strings.Contains(out, "rebase/merge is in progress") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	os.Remove(filepath.Join(gitDir, "MERGE_HEAD"))
	if err := os.WriteFile(filepath.Join(gitDir, "index.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e.setIssues("2026-02-01T00:00:00.000+0000", "TEST-1")
	if code, out := e.pull(t, "--full", "my-issues"); code != 0 || !strings.Contains(out, "index is locked") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if commitCount(t, e.mirror) != 1 {
		t.Fatal("unexpected commit")
	}
}

func TestPullGit_UserCommitBetweenPullsPreserved(t *testing.T) {
	e := newGitPullEnv(t, true)
	e.initRepo(t)
	e.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1")
	e.pull(t, "--full", "my-issues")
	if err := os.WriteFile(filepath.Join(e.mirror, "mine.txt"), []byte("m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, e.mirror, "add", "mine.txt")
	gitOut(t, e.mirror, "commit", "-m", "user work")
	e.setIssues("2026-02-01T00:00:00.000+0000", "TEST-1")
	if code, out := e.pull(t, "--full", "my-issues"); code != 0 {
		t.Fatalf("pull = %d: %s", code, out)
	}
	log := gitOut(t, e.mirror, "log", "--format=%s")
	if !strings.Contains(log, "user work") || strings.Count(log, "sync from") != 2 {
		t.Fatalf("log = %q", log)
	}
	if files := gitOut(t, e.mirror, "show", "--name-only", "--format=", "HEAD"); strings.Contains(files, "mine.txt") {
		t.Fatalf("sync commit included user file: %q", files)
	}
}

func TestPullGit_IssueModeAndDryRunAndOff(t *testing.T) {
	e := newGitPullEnv(t, true)
	e.initRepo(t)
	e.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1")
	n := commitCount(t, e.mirror)
	if code, out := e.pull(t, "--full", "--dry-run", "my-issues"); code != 0 {
		t.Fatalf("dry-run = %d: %s", code, out)
	}
	if commitCount(t, e.mirror) != n {
		t.Fatal("dry-run committed")
	}
	if code, out := e.pull(t, "TEST-1"); code != 0 {
		t.Fatalf("issue pull = %d: %s", code, out)
	}
	if body := gitOut(t, e.mirror, "log", "-1", "--format=%b"); !strings.Contains(body, "issue: TEST-1") {
		t.Fatalf("body = %q", body)
	}

	off := newGitPullEnv(t, false)
	off.initRepo(t)
	off.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1")
	off.pull(t, "--full", "my-issues")
	if commitCount(t, off.mirror) != 1 {
		t.Fatal("committed with git: false")
	}
}

func TestPullGit_NotARepoAndNoIdentity(t *testing.T) {
	e := newGitPullEnv(t, true)
	e.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1")
	code, out := e.pull(t, "--full", "my-issues")
	if code != 0 {
		t.Fatalf("pull = %d: %s", code, out)
	}
	if exec.Command("git", "-C", e.mirror, "rev-parse").Run() == nil {
		return // inside an enclosing repo; nothing more to assert
	}
	if !strings.Contains(out, "not a git repository") || !strings.Contains(out, "git -C") {
		t.Fatalf("out = %q", out)
	}

	e2 := newGitPullEnv(t, true)
	e2.initRepo(t)
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	e2.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1")
	code, out = e2.pull(t, "--full", "my-issues")
	if code != 0 || !strings.Contains(out, "pull ok, git commit failed") || !strings.Contains(out, "git -C") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestPullGit_MirrorInsideLargerRepo(t *testing.T) {
	e := newGitPullEnv(t, true)
	// Repo root is the parent of the mirror dir.
	gitOut(t, e.root, "init", "-b", "main")
	gitOut(t, e.root, "add", "-A")
	gitOut(t, e.root, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(e.root, "outside.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.setIssues("2026-01-01T00:00:00.000+0000", "TEST-1")
	if code, out := e.pull(t, "--full", "my-issues"); code != 0 {
		t.Fatalf("pull = %d: %s", code, out)
	}
	files := gitOut(t, e.root, "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(files, "mirror/issues/TEST-1.md") || strings.Contains(files, "outside.txt") {
		t.Fatalf("files = %q", files)
	}
}

func TestPullGitHelpers(t *testing.T) {
	msg := pullCommitMessage(pullGitCommit{Label: "TEST-1", Host: "h", Archived: []schema.IssueKey{"B-2", "A-1"}, Moved: []schema.IssueKey{"C-3"}}, []string{"TEST-1"}, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	for _, want := range []string{"sync from h at 2026-01-02T03:04:05Z", "issue: TEST-1", "changed: TEST-1", "archived: A-1, B-2", "moved: C-3"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("msg = %q, want %q", msg, want)
		}
	}
	if hostOf("not a url") != "not a url" || hostOf("https://x.example/y") != "x.example" {
		t.Fatal("hostOf")
	}
	var g *pullGit
	g.commit(pullGitCommit{}, time.Now(), nil, nil) // nil receiver is a no-op
	if beginPullGit("", false, nil) != nil || beginPullGit(t.TempDir(), true, nil) != nil {
		t.Fatal("beginPullGit should be nil")
	}
}

func TestPullGit_GoneAndMovedAreCommitted(t *testing.T) {
	e := newGitPullEnv(t, true)
	e.initRepo(t)
	for _, k := range []string{"TEST-1", "TEST-2"} {
		e.fake.SetIssue(k, &schema.Issue{
			Identity:     schema.IssueIdentity{Key: schema.IssueKey(k), Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}},
			MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
			Summary:      "Summary " + k,
		})
		if exit := RunPull([]string{"--project", "test", k}); exit != 0 {
			t.Fatalf("pull %s exit = %d", k, exit)
		}
	}

	e.fake.SetIssue("TEST-1", nil)
	moved := &schema.Issue{
		Identity:     schema.IssueIdentity{Key: "TEST-12", Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}},
		MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
		Summary:      "Summary TEST-2",
	}
	e.fake.SetIssue("TEST-2", moved)

	if exit := RunPull([]string{"--project", "test", "TEST-1"}); exit != 0 {
		t.Fatalf("gone pull exit = %d", exit)
	}
	body := gitOut(t, e.mirror, "show", "--name-status", "--format=%B", "HEAD")
	t.Logf("gone commit:\n%s", body)
	for _, want := range []string{"archived: TEST-1", "issues/TEST-1.md\tissues/_archive/TEST-1.md"} {
		if !strings.Contains(body, want) {
			t.Fatalf("gone commit missing %q:\n%s", want, body)
		}
	}

	if exit := RunPull([]string{"--project", "test", "TEST-2"}); exit != 0 {
		t.Fatalf("moved pull exit = %d", exit)
	}
	body = gitOut(t, e.mirror, "show", "--name-status", "--format=%B", "HEAD")
	t.Logf("moved commit:\n%s", body)
	for _, want := range []string{"moved: TEST-2", "issues/TEST-2.md\tissues/_archive/TEST-2.md", "issues/TEST-12.md"} {
		if !strings.Contains(body, want) {
			t.Fatalf("moved commit missing %q:\n%s", want, body)
		}
	}
	if st := gitOut(t, e.mirror, "status", "--porcelain", "--", "issues"); strings.TrimSpace(st) != "" {
		t.Fatalf("issues dir not clean:\n%s", st)
	}
}
