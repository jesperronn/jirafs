package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

func base42(summary string) *schema.Issue {
	return &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42", Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"}},
		Summary:  summary,
	}
}

func mirrorDirFromHome(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".jirafs", "settings.toml"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`mirror_dir = "([^"]+)"`).FindStringSubmatch(string(data))
	if m == nil {
		t.Fatal("no mirror_dir in settings")
	}
	return m[1]
}

func TestPullWritesBaseSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", base42("Remote summary"))
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return fake, nil })
	_, stderr := withPullTestIO(t)

	if exit := RunPull([]string{"--project", "test", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	mirrorDir := filepath.Join(tmpDir, "mirror")
	entries, err := os.ReadDir(filepath.Join(mirrorDir, ".jirafs", "base"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	t.Logf("ls .jirafs/base:\n%s", strings.Join(names, "\n"))
	if len(names) != 1 || names[0] != "PROJ-42.md" {
		t.Fatalf("base dir = %v", names)
	}
	got, ok := mirror.LoadBase(mirrorDir, "PROJ-42")
	if !ok || got.Summary != "Remote summary" {
		t.Fatalf("LoadBase = %+v, %v", got, ok)
	}
}

func TestPushUpdatesBaseAndFailedPushKeepsOld(t *testing.T) {
	setupLocalPush(t)
	mirrorDir := mirrorDirFromHome(t)
	if err := mirror.SaveBase(mirrorDir, base42("Remote original summary")); err != nil {
		t.Fatal(err)
	}
	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", base42("Remote original summary"))
	fake.SetErr("update", jira.NewHTTPErr(500, "boom"))
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return fake, nil })
	_, _ = withPushIO(t)

	if exit := RunPush([]string{"--project", "test", "--apply", "PROJ-42"}); exit != 1 {
		t.Fatalf("failed push exit = %d, want 1", exit)
	}
	got, ok := mirror.LoadBase(mirrorDir, "PROJ-42")
	if !ok || got.Summary != "Remote original summary" {
		t.Fatalf("failed push changed base: %+v, %v", got, ok)
	}

	fake.SetErr("update", nil)
	if exit := RunPush([]string{"--project", "test", "--apply", "PROJ-42"}); exit != 0 {
		t.Fatalf("push exit = %d, want 0", exit)
	}
	got, ok = mirror.LoadBase(mirrorDir, "PROJ-42")
	if !ok || got.Summary == "Remote original summary" {
		t.Fatalf("successful push did not update base: %+v, %v", got, ok)
	}
}

func TestWriteSyncedIssueBaseError(t *testing.T) {
	dir := t.TempDir()
	mirrorDir := filepath.Join(dir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mirrorDir, ".jirafs"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeSyncedIssue(filepath.Join(dir, "PROJ-42.md"), mirrorDir, base42("x")); err == nil {
		t.Fatal("expected base write error")
	}
}
