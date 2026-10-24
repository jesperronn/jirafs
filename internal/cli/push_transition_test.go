package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

func setupTransitionPush(t *testing.T) *jira.FakeTransport {
	t.Helper()
	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	issue := "---\nkey: PROJ-42\ntype: story\nproject:\n  type: project\n  value: PROJ\nstatus: in progress\n---\n\nSummary: Same summary\n"
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-42.md"), []byte(issue), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := "version = 1\n\n[instances.default]\nbase_url = \"https://example.atlassian.net\"\nauth_type = \"basic\"\n\n[projects.test]\nkey = \"TEST\"\ninstance = \"default\"\nmirror_dir = \"" +
		filepath.Join(tmpDir, "mirror") + "\"\nlocal_dirs = [\"" + localDir + "\"]\n"
	if err := os.MkdirAll(filepath.Join(homeDir, ".jirafs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".jirafs", "settings.toml"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42", Type: "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"}},
		Summary: "Same summary",
		Status:  "Open",
	})
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	return fake
}

func startTransition() []jira.Transition {
	ts := []jira.Transition{{ID: "11", Name: "Start"}, {ID: "21", Name: "Close"}}
	ts[0].To.Name = "In Progress"
	ts[1].To.Name = "Done"
	return ts
}

func TestPushUpdate_StatusTransitionApplied(t *testing.T) {
	fake := setupTransitionPush(t)
	fake.SetTransitions("PROJ-42", startTransition())
	stdout, stderr := withPushIO(t)

	if exit := RunPush([]string{"--project", "test", "--apply", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `transition status: Open → In Progress (via "Start")`) {
		t.Errorf("stdout = %q", stdout.String())
	}
	if got := fake.DoneTransitions(); len(got) != 1 || got[0] != "PROJ-42:11" {
		t.Errorf("transitions done = %v", got)
	}
}

func TestPushUpdate_StatusTransitionDryRun(t *testing.T) {
	fake := setupTransitionPush(t)
	fake.SetTransitions("PROJ-42", startTransition())
	stdout, _ := withPushIO(t)

	if exit := RunPush([]string{"--project", "test", "--dry-run", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d", exit)
	}
	if !strings.Contains(stdout.String(), `(via "Start")`) {
		t.Errorf("stdout = %q", stdout.String())
	}
	if got := fake.DoneTransitions(); len(got) != 0 {
		t.Errorf("dry run performed transitions: %v", got)
	}
}

func TestPushUpdate_StatusNoTransition(t *testing.T) {
	fake := setupTransitionPush(t)
	ts := startTransition()
	fake.SetTransitions("PROJ-42", ts[1:])
	_, stderr := withPushIO(t)

	if exit := RunPush([]string{"--project", "test", "PROJ-42"}); exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	out := stderr.String()
	for _, want := range []string{"no available Jira transition", "Done", "jirafs push PROJ-42 --dry-run"} {
		if !strings.Contains(out, want) {
			t.Errorf("stderr missing %q: %q", want, out)
		}
	}
	if len(fake.DoneTransitions()) != 0 {
		t.Error("transition performed despite no match")
	}
}

func TestPushUpdate_StatusTransitionErrors(t *testing.T) {
	fake := setupTransitionPush(t)
	fake.SetTransitions("PROJ-42", startTransition())
	_, stderr := withPushIO(t)

	fake.SetErr("transitions", jira.NewUnknownErr("boom"))
	if exit := RunPush([]string{"--project", "test", "PROJ-42"}); exit != 1 || !strings.Contains(stderr.String(), "cannot fetch transitions") {
		t.Errorf("fetch error: exit=%d stderr=%q", exit, stderr.String())
	}
	fake.SetErr("transition", jira.NewUnknownErr("boom"))
	if exit := RunPush([]string{"--project", "test", "--apply", "PROJ-42"}); exit != 1 || !strings.Contains(stderr.String(), "status transition") {
		t.Errorf("do error: exit=%d stderr=%q", exit, stderr.String())
	}
}

func TestNoTransitionMessage_None(t *testing.T) {
	if got := noTransitionMessage("K-1", "A", "B", nil, "K-1.md"); !strings.Contains(got, "(none)") {
		t.Errorf("got %q", got)
	}
}
