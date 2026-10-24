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
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

func conflictIssue(summary, desc string) *schema.Issue {
	return &schema.Issue{
		Identity:    schema.IssueIdentity{Key: "PROJ-42", Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"}},
		Summary:     summary,
		Description: desc,
	}
}

// conflictFixture writes a local PROJ-42.md with the given description and
// returns the mirror dir and a fake whose remote has remoteDesc.
func conflictFixture(t *testing.T, localDesc, remoteDesc string) (string, *jira.FakeTransport) {
	t.Helper()
	setupLocalPush(t)
	local := conflictIssue("Shared summary", localDesc)
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(mirrorDirFromHome(t)), "local", "PROJ-42.md"))
	if len(files) != 1 {
		t.Fatalf("local file not found: %v", files)
	}
	if err := os.WriteFile(files[0], []byte(schema.RenderIssue(*local)), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", conflictIssue("Shared summary", remoteDesc))
	client := &writeCountingClient{Client: fake}
	withPushClientFactory(t, func(_ *config.Settings, _ *context.Context, _ string) (jira.Client, error) { return client, nil })
	return mirrorDirFromHome(t), fake
}

func TestPushConflictNamesTheCollidingField(t *testing.T) {
	mirrorDir, fake := conflictFixture(t, "my description", "their description")
	if err := mirror.SaveBase(mirrorDir, conflictIssue("Shared summary", "original description")); err != nil {
		t.Fatal(err)
	}
	stdout, _ := withPushIO(t)
	if exit := RunPush([]string{"--project", "test", "--apply", "PROJ-42"}); exit != 0 {
		t.Fatalf("a skipped conflict is not a failure; exit = %d", exit)
	}
	t.Logf("push output:\n%s", stdout)
	for _, want := range []string{"conflict: description changed locally and remotely", "jirafs diff PROJ-42", "PROJ-42  skipped (conflict)"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("missing %q in:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout.String(), "conflict: summary") {
		t.Errorf("summary must not be reported:\n%s", stdout)
	}
	if got, _ := fake.FetchIssue(stdcontext.Background(), "PROJ-42"); got.Description != "their description" {
		t.Errorf("remote overwritten: %q", got.Description)
	}
}

func TestPushRemoteOnlyChangeIsNotOverwritten(t *testing.T) {
	mirrorDir, fake := conflictFixture(t, "original description", "their description")
	if err := mirror.SaveBase(mirrorDir, conflictIssue("Old summary", "original description")); err != nil {
		t.Fatal(err)
	}
	stdout, _ := withPushIO(t)
	if exit := RunPush([]string{"--project", "test", "--apply", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d: %s", exit, stdout)
	}
	got, _ := fake.FetchIssue(stdcontext.Background(), "PROJ-42")
	if got.Description != "their description" || got.Summary != "Shared summary" {
		t.Errorf("remote = %q / %q; stdout:\n%s", got.Summary, got.Description, stdout)
	}
}

func TestPushMissingBaseSuggestsPull(t *testing.T) {
	_, _ = conflictFixture(t, "my description", "their description")
	stdout, _ := withPushIO(t)
	if exit := RunPush([]string{"--project", "test", "--apply", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d", exit)
	}
	for _, want := range []string{"conflict: description differs locally and remotely", "jirafs pull PROJ-42", "jirafs diff PROJ-42"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("missing %q in:\n%s", want, stdout)
		}
	}
}
