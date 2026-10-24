package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

func ptrString(s string) *string {
	return &s
}

// TestRunPull_NoArgNoScopes verifies that a bare pull without any configured
// scope fails with a copy-pasteable mirror.yml example.
func TestRunPull_NoArgNoScopes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	_, stderr := withPullTestIO(t)
	exit := RunPull([]string{"--project", "test"})
	if exit != 1 {
		t.Fatalf("RunPull([]) = %d, want 1", exit)
	}
	for _, want := range []string{"no scopes configured", "jirafs pull my-issues", "scopes:"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

// TestRunPullIssue_MissingProject verifies that pull KEY without
// a resolvable project returns exit code 1.
func TestRunPullIssue_MissingProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPullTestIO(t)
	// Don't pass --project, and use a non-matching cwd.
	exit := RunPull([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-1"})
	if exit != 1 {
		t.Fatalf("RunPull([\"issue\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestRunPullIssue_Success verifies that pull KEY fetches and
// exports a single issue through the real service path.
func TestRunPullIssue_Success(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-42",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary:      "Test issue summary",
		Description:  "Test issue description",
		Labels:       []string{"bug", "urgent"},
		Assignee:     ptrString("alice"),
		MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
		RemoteMetadata: schema.RemoteMetadata{
			StateFile:     "synced",
			RemoteVersion: "7",
			ContentHash:   "abc123",
		},
	})
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--stdout", "--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunPull([\"issue\", \"--project\", \"test\", \"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
	}

	// Verify the exported output contains the issue key and summary.
	output := stdout.String()
	if !strings.Contains(output, "PROJ-42") {
		t.Fatalf("stdout = %q, want PROJ-42 key", output)
	}
	if !strings.Contains(output, "Test issue summary") {
		t.Fatalf("stdout = %q, want summary", output)
	}

	// Verify round-trip: parse the exported output back.
	parsed, pe := schema.ParseIssue(output)
	if pe != nil {
		t.Fatalf("ParseIssue failed on exported output: %s", pe.Error())
	}
	if parsed.Identity.Key != "PROJ-42" {
		t.Errorf("round-trip key = %q, want PROJ-42", parsed.Identity.Key)
	}
	if parsed.Summary != "Test issue summary" {
		t.Errorf("round-trip summary = %q, want \"Test issue summary\"", parsed.Summary)
	}
}

// TestRunPullIssue_NotFound verifies that exporting a non-existent
// issue returns a not found error.
func TestRunPullIssue_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--project", "test", "PROJ-99"})
	if exit != 1 {
		t.Fatalf("RunPull([\"issue\", \"PROJ-99\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not_found") {
		t.Fatalf("stderr = %q, want not_found error", stderr.String())
	}
}

// TestRunPullIssue_TooManyArgs verifies that pull KEY with
// too many positional arguments returns an error.
func TestRunPullIssue_TooManyArgs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPullTestIO(t)
	exit := RunPull([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-1", "extra"})
	if exit != 2 {
		t.Fatalf("RunPull([\"issue\", \"PROJ-1\", \"extra\"]) = %d, want 2", exit)
	}
	if !strings.Contains(stderr.String(), "too many positional arguments") {
		t.Fatalf("stderr = %q, want too many positional arguments", stderr.String())
	}
}

// TestRunPullIssue_ClientCreationError verifies that pull KEY
// returns an error when the Jira client cannot be created.
func TestRunPullIssue_ClientCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, os.ErrPermission
	})
	_, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--project", "test", "--cwd", filepath.Join(tmpDir, "outside"), "PROJ-1"})
	if exit != 1 {
		t.Fatalf("RunPull([\"issue\", \"PROJ-1\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create Jira client") {
		t.Fatalf("stderr = %q, want client creation error", stderr.String())
	}
}

// TestRunPullIssue_FetchError verifies that pull KEY returns
// an error when the Jira fetch fails.
func TestRunPullIssue_FetchError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetErr("fetch", jira.NewHTTPErr(500, "server error"))
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--project", "test", "--cwd", filepath.Join(tmpDir, "outside"), "PROJ-1"})
	if exit != 1 {
		t.Fatalf("RunPull([\"issue\", \"PROJ-1\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "server error") {
		t.Fatalf("stderr = %q, want server error", stderr.String())
	}
}

// TestRunPullIssue_MinimalIssue verifies that exporting a minimal
// (draft) issue still produces valid output.
func TestRunPullIssue_MinimalIssue(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssue("DRF-1", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "DRF-1",
			Type:    "task",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "DRF"},
		},
		MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
	})
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--stdout", "--project", "test", "--cwd", filepath.Join(tmpDir, "outside"), "DRF-1"})
	if exit != 0 {
		t.Fatalf("RunPull([\"issue\", \"DRF-1\"]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	if output == "" {
		t.Fatal("stdout should not be empty")
	}

	// Verify round-trip.
	parsed, pe := schema.ParseIssue(output)
	if pe != nil {
		t.Fatalf("ParseIssue failed on exported output: %s", pe.Error())
	}
	if parsed.Identity.Key != "DRF-1" {
		t.Errorf("round-trip key = %q, want DRF-1", parsed.Identity.Key)
	}
}

// TestBuildExportClient_Success verifies that newJiraClient
// creates a valid Jira client from settings.
func TestBuildExportClient_Success(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "creds.toml")
	if err := os.WriteFile(credsPath, []byte("email = \"user@example.com\"\napi_token = \"token\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile creds.toml: %v", err)
	}
	settings := &config.Settings{
		Instances: map[string]config.Instance{
			"default": {
				BaseURL:        "https://example.atlassian.net",
				AuthType:       "atlassian_api_token",
				CredentialRefs: []string{"file://" + credsPath},
			},
		},
	}

	client, err := newJiraClient(settings, &context.Context{Instance: "default"}, ".")
	if err != nil {
		t.Fatalf("newJiraClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestRunPullIssue_ResolveContextError verifies that pull KEY
// returns an error when no project can be resolved for the given cwd.
func TestRunPullIssue_ResolveContextError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPullTestIO(t)
	exit := RunPull([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-1"})
	if exit != 1 {
		t.Fatalf("RunPull([\"issue\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestBuildExportClient_CredentialError verifies that newJiraClient
// returns an error when the instance has no credentials configured.
func TestBuildExportClient_CredentialError(t *testing.T) {
	_ = t.TempDir()
	settings := &config.Settings{
		Instances: map[string]config.Instance{
			"default": {
				BaseURL:  "https://example.atlassian.net",
				AuthType: "basic",
				// No CredentialRefs → should fail.
			},
		},
	}

	_, err := newJiraClient(settings, &context.Context{Instance: "default"}, ".")
	if err == nil {
		t.Fatal("expected error for instance with no credentials")
	}
	if !strings.Contains(err.Error(), "no_usable_instance") {
		t.Fatalf("error = %q, want no_usable_instance", err.Error())
	}
}

// TestRunPullIssue_BrowseURL verifies that a Jira browse URL on the
// project's base_url host is accepted in place of an issue key.
func TestRunPullIssue_BrowseURL(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-42",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary:        "URL issue",
		MachineOwned:   schema.MachineOwned{SchemaVersion: "1"},
		RemoteMetadata: schema.RemoteMetadata{StateFile: "synced", RemoteVersion: "1", ContentHash: "h"},
	})
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--stdout", "--project", "test", "https://example.atlassian.net/browse/PROJ-42"})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "URL issue") {
		t.Fatalf("stdout = %q, want summary", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exit = RunPull([]string{"--project", "test", "https://evil.example.com/browse/PROJ-42"})
	if exit != 1 {
		t.Fatalf("wrong-host exit = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "jirafs pull KEY --project test") {
		t.Fatalf("stderr = %q, want copy-pasteable hint", stderr.String())
	}
}

func exportFakeIssue(key string) *jira.FakeTransport {
	fake := jira.NewFakeTransport()
	fake.SetIssue(key, &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     schema.IssueKey(key),
			Type:    "task",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary:        "File issue",
		MachineOwned:   schema.MachineOwned{SchemaVersion: "1"},
		RemoteMetadata: schema.RemoteMetadata{StateFile: "synced", RemoteVersion: "1", ContentHash: "h"},
	})
	return fake
}

// TestRunPullIssue_WritesFile verifies the default mode writes
// <local_dirs[0]>/<KEY>.md (creating the missing directory) and prints the
// absolute path.
func TestRunPullIssue_WritesFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	fake := exportFakeIssue("PROJ-7")
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)

	// The local dir does not exist yet; it must be created.
	localDir := filepath.Join(tmpDir, "local")
	if _, err := os.Stat(localDir); err == nil {
		t.Fatal("local dir unexpectedly exists")
	}

	exit := RunPull([]string{"--project", "test", "PROJ-7"})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	dest := filepath.Join(localDir, "PROJ-7.md")
	want := "wrote PROJ-7 → " + dest + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if !filepath.IsAbs(dest) {
		t.Fatalf("dest %q not absolute", dest)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "File issue") {
		t.Fatalf("file = %q, want summary", string(data))
	}
}

// TestRunPullIssue_StdoutWritesNoFile verifies --stdout prints the document
// and writes no file.
func TestRunPullIssue_StdoutWritesNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	fake := exportFakeIssue("PROJ-8")
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--stdout", "--project", "test", "PROJ-8"})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "File issue") || strings.HasPrefix(stdout.String(), "wrote ") {
		t.Fatalf("stdout = %q, want document", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "local")); err == nil {
		t.Fatal("--stdout must not create the local dir or file")
	}
}

// TestRunPullIssue_NoLocalDirs verifies a project without local_dirs fails
// with a hint in file mode.
func TestRunPullIssue_NoLocalDirs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	fake := exportFakeIssue("PROJ-9")
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPullTestIO(t)
	// Make local dir creation impossible: a regular file occupies its path.
	if err := os.WriteFile(filepath.Join(tmpDir, "local"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	exit := RunPull([]string{"--project", "test", "PROJ-9"})
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "create local directory") {
		t.Fatalf("stderr = %q, want create local directory error", stderr.String())
	}
}

func TestRunPullIssue_IncludesCommentsAndWarnsOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42", Type: "story"},
		Summary:  "S",
	})
	fake.SetComments("PROJ-42", []schema.Comment{
		{Author: "bob", Created: "2024-02-01T00:00:00Z", Body: "newer"},
		{Author: "alice", Created: "2024-01-01T00:00:00Z", Body: "older"},
	})
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)
	if exit := RunPull([]string{"--stdout", "--project", "test", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "## Comments") || strings.Index(out, "### bob") > strings.Index(out, "### alice") {
		t.Errorf("comments missing or misordered:\n%s", out)
	}

	// A failed comments fetch is a warning, not an error.
	fake.SetIssue("PROJ-42", &schema.Issue{Identity: schema.IssueIdentity{Key: "PROJ-42", Type: "story"}, Summary: "S"})
	fake.SetErr("comments", errors.New("comments down"))
	stdout.Reset()
	stderr.Reset()
	if exit := RunPull([]string{"--stdout", "--project", "test", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d, want 0 on comments failure; stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "warning: cannot fetch comments") || strings.Contains(stdout.String(), "## Comments") {
		t.Errorf("stderr = %q stdout = %q", stderr.String(), stdout.String())
	}
}

// TestRunPullIssue_NoLocalDirsSuggestsStdout verifies that exporting to a file when the
// project has no local_dirs fails with exit 1 and points the user at --stdout.
func TestRunPullIssue_NoLocalDirsSuggestsStdout(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	credsPath := filepath.Join(tmpDir, "creds.toml")
	if err := os.WriteFile(credsPath, []byte("email = \"user@example.com\"\napi_token = \"token\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile creds: %v", err)
	}
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "atlassian_api_token"
credential_refs = ["file://` + credsPath + `"]

[projects.test]
key = "PROJ"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-42",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary:      "No local dir",
		MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
	})
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPull(issue PROJ-42) = %d, want 1; stdout = %q", exit, stdout.String())
	}
	if !strings.Contains(stderr.String(), "no local_dirs configured") || !strings.Contains(stderr.String(), "--stdout") {
		t.Errorf("stderr = %q, want no local_dirs error suggesting --stdout", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}
