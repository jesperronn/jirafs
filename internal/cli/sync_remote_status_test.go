package cli

import (
	stdcontext "context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

// setupStatusEnv writes a local PROJ-42 file and a fake remote with the given
// status, and wires both the sync and plan client factories to the fake.
func setupStatusEnv(t *testing.T, localFrontmatterExtra, localSummary, remoteStatus string) (*jira.FakeTransport, string) {
	t.Helper()
	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(localDir, "PROJ-42.md")
	issue := "---\nkey: PROJ-42\ntype: story\nproject:\n  type: project\n  value: PROJ\n" + localFrontmatterExtra + "---\n\nSummary: " + localSummary + "\n"
	if err := os.WriteFile(localPath, []byte(issue), 0o644); err != nil {
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
		Summary: "Old summary",
		Status:  remoteStatus,
	})
	factory := func(*config.Settings, *context.Context, string) (jira.Client, error) { return fake, nil }
	withSyncClientFactory(t, factory)
	withPlanClientFactory(t, factory)
	return fake, localPath
}

func TestRunSync_UnchangedStatusIsNotTransitioned(t *testing.T) {
	fake, _ := setupStatusEnv(t, "status: 'Open'\n", "New summary", "Open")
	stdout, stderr := withSyncTestIO(t)

	if exit := RunSync([]string{"--project", "test", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 operation(s)") || strings.Contains(stdout.String(), "status") {
		t.Errorf("stdout = %q, want one summary op and no status op", stdout.String())
	}
	if n := fake.FetchTransitionsCalls(); n != 0 {
		t.Errorf("FetchTransitions called %d times, want 0", n)
	}
	if got := fake.DoneTransitions(); len(got) != 0 {
		t.Errorf("transitions done = %v, want none", got)
	}
}

func TestRunSync_StatusCaseInsensitiveNoOp(t *testing.T) {
	fake, _ := setupStatusEnv(t, "status: open\n", "New summary", "Open")
	withSyncTestIO(t)
	if exit := RunSync([]string{"--project", "test", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d", exit)
	}
	if fake.FetchTransitionsCalls() != 0 {
		t.Error("transitions fetched for case-only difference")
	}
}

func TestRunSync_UnknownRemoteStatusNeverTransitions(t *testing.T) {
	fake, _ := setupStatusEnv(t, "status: 'Open'\n", "New summary", "")
	_, stderr := withSyncTestIO(t)
	if exit := RunSync([]string{"--project", "test", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if fake.FetchTransitionsCalls() != 0 || len(fake.DoneTransitions()) != 0 {
		t.Error("transition attempted with unknown remote status")
	}
	if !strings.Contains(stderr.String(), "remote status of PROJ-42 is unknown") {
		t.Errorf("stderr = %q, want warning", stderr.String())
	}
}

func TestRunPlan_UnchangedStatusShowsNoStatusOp(t *testing.T) {
	setupStatusEnv(t, "status: 'Open'\n", "New summary", "Open")
	stdout, stderr := withPlanTestIO(t)
	if exit := RunPlan([]string{"--project", "test"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "1 operation(s)") || !strings.Contains(out, "summary") || strings.Contains(out, "status") {
		t.Errorf("stdout = %q", out)
	}
}

func gatedTransition() []jira.Transition {
	ts := []jira.Transition{{ID: "21", Name: "Close", Fields: map[string]jira.TransitionField{
		"customfield_900": {Required: true, Name: "Root cause"},
	}}}
	ts[0].To.Name = "Done"
	return ts
}

func TestRunSync_GatedTransitionWithoutValueBlocks(t *testing.T) {
	fake, localPath := setupStatusEnv(t, "status: Done\n", "New summary", "Open")
	fake.SetTransitions("PROJ-42", gatedTransition())
	stdout, stderr := withSyncTestIO(t)
	before, _ := os.ReadFile(localPath)

	for _, args := range [][]string{{"--project", "test", "PROJ-42"}, {"--project", "test", "--dry-run", "PROJ-42"}} {
		if exit := RunSync(args); exit != 1 {
			t.Fatalf("%v: exit = %d, want 1", args, exit)
		}
	}
	for _, want := range []string{`"Close"`, "customfield_900 (Root cause)", "jirafs sync PROJ-42 --dry-run", "nothing was written"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr missing %q: %q", want, stderr.String())
		}
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if len(fake.DoneTransitions()) != 0 {
		t.Error("transition performed")
	}
	if got, _ := fake.FetchIssue(stdcontext.Background(), "PROJ-42"); got.Summary != "Old summary" {
		t.Errorf("remote summary changed to %q", got.Summary)
	}
	if after, _ := os.ReadFile(localPath); string(after) != string(before) {
		t.Error("local file rewritten")
	}
}

func TestRunSync_GatedTransitionWithValueSendsFields(t *testing.T) {
	fake, _ := setupStatusEnv(t, "status: Done\ncustom_fields:\n  customfield_900: Bug in login\n", "New summary", "Open")
	fake.SetTransitions("PROJ-42", gatedTransition())
	_, stderr := withSyncTestIO(t)
	if exit := RunSync([]string{"--project", "test", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if got := fake.DoneTransitions(); len(got) != 1 || got[0] != "PROJ-42:21" {
		t.Fatalf("done = %v", got)
	}
	if f := fake.DoneFields("PROJ-42:21"); f["customfield_900"] != "Bug in login" {
		t.Errorf("fields = %v", f)
	}
}
