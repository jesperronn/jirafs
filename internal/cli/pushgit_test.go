package cli

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

func syncedDoc(key, summary string) string {
	doc := "---\nkey: '" + key + "'\ntype: 'story'\nproject: 'project:TEST'\nstate: 'synced'\n" +
		"remote_version: '1'\nupdated: '2026-01-01T00:00:00Z'\nsummary: '" + summary + "'\n---\n## Description\nbody\n\n"
	return schema.StampSyncedHash(doc)
}

// newGitPushEnv is a git-backed mirror with synced TEST-11, TEST-12 and TEST-13
// committed, and the fake Jira knowing TEST-11 and TEST-12 (not TEST-13).
func newGitPushEnv(t *testing.T, gitOn bool) *gitPullEnv {
	t.Helper()
	e := newGitPullEnv(t, gitOn)
	for _, k := range []string{"TEST-11", "TEST-12", "TEST-13"} {
		if err := os.WriteFile(filepath.Join(e.local, k+".md"), []byte(syncedDoc(k, "original")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, k := range []string{"TEST-11", "TEST-12"} {
		e.fake.SetIssue(k, &schema.Issue{
			Identity: schema.IssueIdentity{Key: schema.IssueKey(k), Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}},
			Summary:  "original", Description: "body",
		})
	}
	if err := os.WriteFile(filepath.Join(e.mirror, ".gitignore"), []byte(strings.Join(gitignoreEntries, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.initRepo(t)
	return e
}

func (e *gitPullEnv) edit(t *testing.T, key string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(e.local, key+".md"), []byte(strings.Replace(syncedDoc(key, "original"), "'original'", "'edited'", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e *gitPullEnv) push(t *testing.T, args ...string) (int, string) {
	t.Helper()
	out, errBuf := withPushTestIO(t, e.fake)
	code := RunPush(append([]string{"--project", "test"}, args...))
	return code, out.String() + errBuf.String()
}

func TestPushGit_SingleUpdateCommits(t *testing.T) {
	e := newGitPushEnv(t, true)
	e.edit(t, "TEST-12")
	n := commitCount(t, e.mirror)
	code, out := e.push(t, "TEST-12", "--apply")
	if code != 0 {
		t.Fatalf("push = %d: %s", code, out)
	}
	if commitCount(t, e.mirror) != n+1 {
		t.Fatalf("no commit: %s", out)
	}
	subj := gitOut(t, e.mirror, "log", "-1", "--format=%s")
	if strings.TrimSpace(subj) != "push to example.atlassian.net at 2026-01-02T03:04:05Z" {
		t.Fatalf("subject = %q", subj)
	}
	if body := gitOut(t, e.mirror, "log", "-1", "--format=%b"); !strings.Contains(body, "TEST-12: fields summary") || strings.Contains(body, "not pushed") {
		t.Fatalf("body = %q", body)
	}
	if files := gitOut(t, e.mirror, "show", "--name-only", "--format=", "HEAD"); strings.TrimSpace(files) != "issues/TEST-12.md" {
		t.Fatalf("files = %q", files)
	}
	t.Log(gitOut(t, e.mirror, "log", "-2", "--stat"))
	if !strings.Contains(out, "committed 1 issue(s) to git") {
		t.Fatalf("out = %q", out)
	}
}

func TestPushGit_UneditedStatusAndOtherDirtyFilesUntouched(t *testing.T) {
	e := newGitPushEnv(t, true)
	e.edit(t, "TEST-12")
	e.edit(t, "TEST-13") // edited but not pushed
	code, out := e.push(t, "TEST-12", "--apply")
	if code != 0 {
		t.Fatalf("push = %d: %s", code, out)
	}
	if st := gitOut(t, e.mirror, "status", "--porcelain"); !strings.Contains(st, " M issues/TEST-13.md") || strings.Contains(st, "TEST-12") {
		t.Fatalf("status = %q", st)
	}
}

func TestPushGit_TransitionSummary(t *testing.T) {
	e := newGitPushEnv(t, true)
	e.fake.SetIssue("TEST-12", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "TEST-12", Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}},
		Summary:  "original", Description: "body", Status: "To Do",
	})
	tr := jira.Transition{ID: "31", Name: "Finish"}
	tr.To.Name = "Done"
	e.fake.SetTransitions("TEST-12", []jira.Transition{tr})
	doc := strings.Replace(syncedDoc("TEST-12", "edited"), "state: 'synced'", "state: 'synced'\nstatus: 'Done'", 1)
	if err := os.WriteFile(filepath.Join(e.local, "TEST-12.md"), []byte(schema.StampSyncedHash(doc)), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := e.push(t, "TEST-12", "--apply"); code != 0 {
		t.Fatalf("push = %d: %s", code, out)
	}
	if body := gitOut(t, e.mirror, "log", "-1", "--format=%b"); !strings.Contains(body, "TEST-12: fields summary; transition To Do -> Done") {
		t.Fatalf("body = %q", body)
	}
}

func TestPushGit_CreateFromDraftCommitsRename(t *testing.T) {
	e := newGitPushEnv(t, true)
	draft := filepath.Join(e.local, "dark-mode.md")
	if err := os.WriteFile(draft, []byte(filledStory), 0o644); err != nil {
		t.Fatal(err)
	}
	// A tracked draft: the removal must be committed with the new file.
	gitOut(t, e.mirror, "add", "-A")
	gitOut(t, e.mirror, "commit", "-m", "draft")
	code, out := e.push(t, draft, "--apply")
	if code != 0 {
		t.Fatalf("push = %d: %s", code, out)
	}
	files := gitOut(t, e.mirror, "show", "--name-status", "--format=", "HEAD")
	if !strings.Contains(files, "issues/dark-mode.md") || !regexp.MustCompile(`A\s+issues/TEST-\d+\.md`).MatchString(files) {
		t.Fatalf("files = %q", files)
	}
	if body := gitOut(t, e.mirror, "log", "-1", "--format=%b"); !regexp.MustCompile(`TEST-\d+: created TEST-\d+ \(from dark-mode\.md\)`).MatchString(body) {
		t.Fatalf("body = %q", body)
	}
	if st := gitOut(t, e.mirror, "status", "--porcelain"); strings.TrimSpace(st) != "" {
		t.Fatalf("tree dirty: %q", st)
	}
}

func TestPushGit_CommitsMirrorYMLWhenJirafsChangedIt(t *testing.T) {
	e := newGitPushEnv(t, true)
	remote, _ := e.fake.FetchIssue(context.Background(), "TEST-12")
	remote.Context.Updated = "2026-03-01T00:00:00Z"
	e.edit(t, "TEST-12")
	if code, out := e.push(t, "TEST-12", "--apply", "--write-local"); code != 0 {
		t.Fatalf("push = %d: %s", code, out)
	}
	files := gitOut(t, e.mirror, "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(files, "issues/TEST-12.md") || !strings.Contains(files, "mirror.yml") {
		t.Fatalf("files = %q", files)
	}
	if st := gitOut(t, e.mirror, "status", "--porcelain"); strings.TrimSpace(st) != "" {
		t.Fatalf("tree dirty: %q", st)
	}
}

func TestPushGit_UntrackedDraftCreate(t *testing.T) {
	e := newGitPushEnv(t, true)
	draft := filepath.Join(e.local, "dark-mode.md")
	if err := os.WriteFile(draft, []byte(filledStory), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := e.push(t, draft, "--apply"); code != 0 {
		t.Fatalf("push = %d: %s", code, out)
	}
	files := gitOut(t, e.mirror, "show", "--name-only", "--format=", "HEAD")
	if strings.Contains(files, "dark-mode.md") || !regexp.MustCompile(`issues/TEST-\d+\.md`).MatchString(files) {
		t.Fatalf("files = %q", files)
	}
}

func TestPushGit_MixedRunPartialCommit(t *testing.T) {
	e := newGitPushEnv(t, true)
	e.edit(t, "TEST-12")
	e.edit(t, "TEST-13") // not in fake Jira: fails
	if err := os.WriteFile(filepath.Join(e.local, "dark-mode.md"), []byte(filledStory), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := e.push(t, "--apply")
	if code != 1 {
		t.Fatalf("push = %d: %s", code, out)
	}
	if !strings.Contains(out, "committed 2 of 3; failed: TEST-13") {
		t.Fatalf("out = %q", out)
	}
	msg := gitOut(t, e.mirror, "log", "-1", "--format=%B")
	t.Logf("mixed-run commit message:\n%s", msg)
	for _, want := range []string{"push to example.atlassian.net", "TEST-12: fields summary", "created TEST-", "not pushed: TEST-13 ("} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q: %q", want, msg)
		}
	}
	if files := gitOut(t, e.mirror, "show", "--name-only", "--format=", "HEAD"); strings.Contains(files, "TEST-13.md") {
		t.Fatalf("failed issue committed: %q", files)
	}
	if st := gitOut(t, e.mirror, "status", "--porcelain"); !strings.Contains(st, " M issues/TEST-13.md") {
		t.Fatalf("status = %q", st)
	}
}

func TestPushGit_NoCommitCases(t *testing.T) {
	e := newGitPushEnv(t, true)
	e.edit(t, "TEST-12")
	n := commitCount(t, e.mirror)
	if code, out := e.push(t, "TEST-12"); code != 0 {
		t.Fatalf("dry run = %d: %s", code, out)
	}
	if commitCount(t, e.mirror) != n {
		t.Fatal("dry run committed")
	}
	// Nothing changed remotely: no commit.
	if code, out := e.push(t, "TEST-11", "--apply"); code != 0 || !strings.Contains(out, "no changes needed") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if commitCount(t, e.mirror) != n {
		t.Fatal("no-op push committed")
	}

	off := newGitPushEnv(t, false)
	off.edit(t, "TEST-12")
	m := commitCount(t, off.mirror)
	if code, out := off.push(t, "TEST-12", "--apply"); code != 0 {
		t.Fatalf("push = %d: %s", code, out)
	}
	if commitCount(t, off.mirror) != m {
		t.Fatal("committed with git: false")
	}
}

func TestPushGit_UserEditsToPushedFileCommittedInFull(t *testing.T) {
	e := newGitPushEnv(t, true)
	e.edit(t, "TEST-12") // uncommitted edit that is the thing being pushed
	if err := os.WriteFile(filepath.Join(e.mirror, "notes.txt"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := e.push(t, "TEST-12", "--apply"); code != 0 {
		t.Fatalf("push = %d: %s", code, out)
	}
	if out := gitOut(t, e.mirror, "show", "HEAD:issues/TEST-12.md"); !strings.Contains(out, "edited") {
		t.Fatalf("committed file = %q", out)
	}
	if st := gitOut(t, e.mirror, "status", "--porcelain"); !strings.Contains(st, "?? notes.txt") {
		t.Fatalf("status = %q", st)
	}
}

func TestPushGit_DirtyMirrorYMLLeftAlone(t *testing.T) {
	e := newGitPushEnv(t, true)
	yml := filepath.Join(e.mirror, "mirror.yml")
	data, _ := os.ReadFile(yml)
	if err := os.WriteFile(yml, append(data, []byte("# user note\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	draft := filepath.Join(e.local, "dark-mode.md")
	if err := os.WriteFile(draft, []byte(filledStory), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := e.push(t, draft, "--apply"); code != 0 {
		t.Fatalf("push = %d: %s", code, out)
	}
	if files := gitOut(t, e.mirror, "show", "--name-only", "--format=", "HEAD"); strings.Contains(files, "mirror.yml") {
		t.Fatalf("committed user-edited mirror.yml: %q", files)
	}
	if st := gitOut(t, e.mirror, "status", "--porcelain"); !strings.Contains(st, " M mirror.yml") {
		t.Fatalf("status = %q", st)
	}
}

func TestPushGit_SkipsAndNeverFailsPush(t *testing.T) {
	e := newGitPushEnv(t, true)
	e.edit(t, "TEST-12")
	gitOut(t, e.mirror, "checkout", "--detach")
	n := commitCount(t, e.mirror)
	code, out := e.push(t, "TEST-12", "--apply")
	if code != 0 || !strings.Contains(out, "detached HEAD") || commitCount(t, e.mirror) != n {
		t.Fatalf("code=%d out=%q", code, out)
	}

	e2 := newGitPushEnv(t, true)
	e2.edit(t, "TEST-12")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	code, out = e2.push(t, "TEST-12", "--apply")
	if code != 0 || !strings.Contains(out, "push ok, git commit failed") || !strings.Contains(out, "git -C") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestPushGitHelpers(t *testing.T) {
	var g *gitTracker
	g.commitPush(nil, "h", time.Now(), nil, nil) // nil receiver is a no-op
	if got := opsSummary(nil, "", nil); got != "" {
		t.Fatalf("got %q", got)
	}
}
