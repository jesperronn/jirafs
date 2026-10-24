package cli

import (
	"bytes"
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

// TestPushUpdate_UnknownSubcommand verifies that RunPush with an unknown
// subcommand (not "help") returns exit code 1.
func TestPushUpdate_UnknownSubcommand(t *testing.T) {
	exit := RunPush([]string{"bogus"})
	if exit != 1 {
		t.Errorf("RunPush([\"bogus\"]) = %d, want 1", exit)
	}
}

// TestPushUpdate_Help verifies that "help" returns exit code 0.
func TestPushUpdate_Help(t *testing.T) {
	exit := RunPush([]string{"help"})
	if exit != 0 {
		t.Errorf("RunPush([\"help\"]) = %d, want 0", exit)
	}
}

func withPushIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := pushStdout
	oldStderr := pushStderr
	pushStdout = stdout
	pushStderr = stderr
	t.Cleanup(func() {
		pushStdout = oldStdout
		pushStderr = oldStderr
	})
	return stdout, stderr
}

func withPushClientFactory(t *testing.T, factory func(*config.Settings, *context.Context, string) (jira.Client, error)) {
	t.Helper()
	oldFactory := pushClientFactory
	pushClientFactory = func(s *config.Settings, c *context.Context, name string) (jira.Client, error) {
		client, err := factory(s, c, name)
		if fake, ok := client.(*jira.FakeTransport); ok && err == nil {
			seedBases(c, fake)
		}
		return client, err
	}
	t.Cleanup(func() {
		pushClientFactory = oldFactory
	})
}

// setupLocalPush writes settings whose local_dirs holds PROJ-42.md (summary
// "Local modified summary") and points HOME at them.
func setupLocalPush(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	issue := "---\nkey: PROJ-42\ntype: story\nproject:\n  type: project\n  value: PROJ\n---\n\nSummary: Local modified summary\n"
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
}

// TestPushUpdate_FailureIsolation verifies that one failing target does not
// stop the others and that the summary names each result.
func TestPushUpdate_FailureIsolation(t *testing.T) {
	setupLocalPush(t)
	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42", Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"}},
		Summary:  "Remote original summary",
	})
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return fake, nil })
	stdout, stderr := withPushIO(t)

	exit := RunPush([]string{"--project", "test", "PROJ-1", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("exit = %d, want 1; stderr = %q", exit, stderr.String())
	}
	for _, want := range []string{"PROJ-1  failed  issue PROJ-1 not found in local directories", "PROJ-42  ok  would apply 1 operation(s)", "1 ok, 0 skipped, 1 failed"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

// TestPushUpdate_NoProjectResolved verifies that sync without a resolvable
// project returns exit code 1.
func TestPushUpdate_NoProjectResolved(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPushIO(t)
	exit := RunPush([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPush([\"--project\", \"test\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestPushUpdate_NoLocalIssue verifies that sync returns an error when
// the local issue file is not found.
func TestPushUpdate_NoLocalIssue(t *testing.T) {
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
		Summary: "Remote issue",
	})
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPushIO(t)

	exit := RunPush([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPush([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in local directories") {
		t.Fatalf("stderr = %q, want not found in local directories", stderr.String())
	}
}

// TestPushUpdate_Success_NoChanges verifies that sync reports no changes
// when local and remote match.
func TestPushUpdate_Success_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a local issue file.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	issueContent := `---
key: PROJ-42
type: story
project:
  type: project
  value: PROJ
summary: Remote issue
---

Summary: Remote issue
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-42.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create settings with local_dirs pointing to the local directory.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + localDir + `"]
`
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
		Summary: "Remote issue",
	})
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPushIO(t)

	exit := RunPush([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunPush([\"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "no changes needed") {
		t.Fatalf("stdout = %q, want \"no changes needed\"", output)
	}
	if !strings.Contains(output, "PROJ-42") {
		t.Fatalf("stdout = %q, want PROJ-42 key", output)
	}
}

// TestPushUpdate_Success_WithChanges verifies that sync reports operations
// and pushes the updated remote when local and remote differ.
func TestPushUpdate_Success_WithChanges(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a local issue file with a different summary.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	issueContent := `---
key: PROJ-42
type: story
project:
  type: project
  value: PROJ
---

Summary: Local modified summary
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-42.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create settings with local_dirs pointing to the local directory.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + localDir + `"]
`
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
		Summary: "Remote original summary",
	})
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPushIO(t)

	exit := RunPush([]string{"--project", "test", "--apply", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunPush([\"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "1 operation(s)") {
		t.Fatalf("stdout = %q, want 1 operation(s)", output)
	}
	if !strings.Contains(output, "PROJ-42") {
		t.Fatalf("stdout = %q, want PROJ-42 key", output)
	}
	if !strings.Contains(output, "summary") {
		t.Fatalf("stdout = %q, want summary field", output)
	}
	if !strings.Contains(output, "successfully") {
		t.Fatalf("stdout = %q, want successfully message", output)
	}
}

// TestPushUpdate_FetchError verifies that sync returns an error when
// the Jira fetch fails.
func TestPushUpdate_FetchError(t *testing.T) {
	setupLocalPush(t)

	fake := jira.NewFakeTransport()
	fake.SetErr("fetch", jira.NewHTTPErr(500, "server error"))
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPushIO(t)

	exit := RunPush([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPush([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "server error") {
		t.Fatalf("stderr = %q, want server error", stderr.String())
	}
}

// TestPushUpdate_NotFound verifies that sync returns a not found error
// when the remote issue does not exist.
func TestPushUpdate_NotFound(t *testing.T) {
	setupLocalPush(t)

	fake := jira.NewFakeTransport()
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPushIO(t)

	exit := RunPush([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPush([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not_found") {
		t.Fatalf("stderr = %q, want not_found error", stderr.String())
	}
}

// TestPushUpdate_ClientCreationError verifies that sync returns an error
// when the Jira client cannot be created.
func TestPushUpdate_ClientCreationError(t *testing.T) {
	setupLocalPush(t)

	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, os.ErrPermission
	})
	_, stderr := withPushIO(t)

	exit := RunPush([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPush([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create Jira client") {
		t.Fatalf("stderr = %q, want client creation error", stderr.String())
	}
}

// TestPushUpdate_LocalIssueUnparseable verifies that sync skips files that
// cannot be parsed as issues.
func TestPushUpdate_LocalIssueUnparseable(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a non-issue file.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-42.md"), []byte("not an issue file"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
		Summary: "Remote issue",
	})
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPushIO(t)

	exit := RunPush([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPush([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in local directories") {
		t.Fatalf("stderr = %q, want not found in local directories", stderr.String())
	}
}

// TestPushUpdate_UpdateIssueError verifies that sync returns an error
// when the Jira update fails after a successful plan.
func TestPushUpdate_UpdateIssueError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a local issue file with a different summary.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	issueContent := `---
key: PROJ-42
type: story
project:
  type: project
  value: PROJ
---

Summary: Local modified summary
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-42.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create settings with local_dirs pointing to the local directory.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + localDir + `"]
`
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
		Summary: "Remote original summary",
	})
	// Configure the fake to fail on update.
	fake.SetErr("update", jira.NewHTTPErr(500, "update server error"))
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPushIO(t)

	exit := RunPush([]string{"--project", "test", "--apply", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPush([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot update remote issue") {
		t.Fatalf("stderr = %q, want cannot update remote issue", stderr.String())
	}
}

// TestPushUpdate_Success_WithChangesAndApply verifies that sync with --apply
// writes the updated remote back to the local file system.
func TestPushUpdate_Success_WithChangesAndApply(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a local issue file with a different summary.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	issueContent := `---
key: PROJ-42
type: story
project:
  type: project
  value: PROJ
summary: Local modified summary
---
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-42.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create settings with local_dirs pointing to the local directory.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + localDir + `"]
`
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
		Summary: "Remote original summary",
	})
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPushIO(t)

	exit := RunPush([]string{"--project", "test", "--apply", "--write-local", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunPush([\"--project\", \"test\", \"--apply\", \"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "successfully") {
		t.Fatalf("stdout = %q, want successfully message", output)
	}

	// Verify the local file was updated with the local summary (which was pushed to remote).
	updatedContent, err := os.ReadFile(filepath.Join(localDir, "PROJ-42.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(updatedContent), "Local modified summary") {
		t.Fatalf("updated file should contain local summary (pushed to remote), got: %s", string(updatedContent))
	}
}

// TestPushUpdate_ResolveContextError verifies that sync returns an error
// when no project can be resolved for the given cwd.
func TestPushUpdate_ResolveContextError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPushIO(t)
	exit := RunPush([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPush([\"--project\", \"test\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestResolvePushContext_NoProject verifies that resolvePushContext
// returns false when no project can be resolved for the given cwd.
func TestResolvePushContext_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	_, stderr := withPushIO(t)

	ctx, ok := resolvePushContext(settings, "", filepath.Join(tmpDir, "outside"))
	if ok {
		t.Fatal("expected unresolved project")
	}
	if ctx != nil {
		t.Fatalf("expected nil context, got %#v", ctx)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Available projects:") {
		t.Fatalf("stderr = %q, want candidate list", stderr.String())
	}
}

// TestBuildSyncClient_Success verifies that newJiraClient
// creates a valid Jira client from settings.
func TestBuildSyncClient_Success(t *testing.T) {
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

// TestBuildSyncClient_CredentialError verifies that newJiraClient
// returns an error when the instance has no credentials configured.
func TestBuildSyncClient_CredentialError(t *testing.T) {
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

// TestPushUpdate_NothingToPush verifies that push without arguments reports
// a clean tree and writes nothing.
func TestPushUpdate_NothingToPush(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	built := false
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		built = true
		return nil, nil
	})
	stdout, stderr := withPushIO(t)
	exit := RunPush([]string{"--project", "test", "--cwd", tmpDir})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "nothing to push") || built {
		t.Fatalf("stdout = %q, built = %v", stdout.String(), built)
	}
}

// TestReadLocalIssueReturnsFullPath guards against returning the bare file
// name, which made --apply write PROJ-42.md into the current directory.
func TestReadLocalIssueReturnsFullPath(t *testing.T) {
	localDir := t.TempDir()
	writeIssue(t, localDir, "PROJ-42.md", "---\nkey: PROJ-42\ntype: story\nproject:\n  type: project\n  value: PROJ\n---\n\nSummary: S\n")
	settings := &config.Settings{Projects: map[string]config.Project{
		"test": {LocalDirs: []string{localDir}},
	}}
	_, path, err := readLocalIssue(&context.Context{Name: "test"}, settings, "PROJ-42")
	if err != nil {
		t.Fatalf("readLocalIssue: %v", err)
	}
	if want := filepath.Join(localDir, "PROJ-42.md"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

// ctxClient wraps a jira.Client, failing reads with the context error like a
// real HTTP client does, and counting write calls.
type ctxClient struct {
	jira.Client
	writes int
}

func (c *ctxClient) FetchIssue(ctx stdcontext.Context, key string) (*schema.Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, jira.NewTransportError("request failed: " + err.Error())
	}
	return c.Client.FetchIssue(ctx, key)
}

func (c *ctxClient) UpdateIssue(ctx stdcontext.Context, key string, issue *schema.Issue) (*schema.Issue, error) {
	c.writes++
	return c.Client.UpdateIssue(ctx, key, issue)
}

func TestPushTarget_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	issueContent := "---\nkey: PROJ-42\ntype: story\nproject:\n  type: project\n  value: PROJ\n---\n\nSummary: Local modified summary\n"
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-42.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	settings := &config.Settings{
		Version:   1,
		Instances: map[string]config.Instance{"default": {BaseURL: "https://example.atlassian.net", AuthType: "basic"}},
		Projects: map[string]config.Project{"test": {
			Key: "TEST", Instance: "default", MirrorDir: filepath.Join(tmpDir, "mirror"), LocalDirs: []string{localDir},
		}},
	}

	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42", Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"}},
		Summary:  "Remote original summary",
	})
	client := &ctxClient{Client: fake}
	withPushClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return client, nil
	})
	_, stderr := withPushIO(t)

	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	run := &pushRun{cmdCtx: ctx, settings: settings, ctx: &context.Context{Name: "test", Key: "TEST", Instance: "default"}, opts: pushOptions{apply: true, writeLocal: true}}
	res := run.pushTarget("PROJ-42", "")
	if res.status != pushFailed {
		t.Fatalf("pushTarget with cancelled context = %+v, want failed; stderr = %q", res, stderr.String())
	}
	if client.writes != 0 {
		t.Fatalf("Jira writes = %d, want 0", client.writes)
	}
	if !strings.Contains(stderr.String(), "canceled") {
		t.Fatalf("stderr = %q, want cancellation message", stderr.String())
	}
}

// seedBases stores every issue of fake as the base snapshot in the mirror of
// ctx, as an earlier pull would have, unless a base already exists. Tests that
// hand-write local files rely on it so the remote counts as unchanged since
// the last pull.
func seedBases(ctx *context.Context, fake *jira.FakeTransport) {
	if ctx == nil {
		return
	}
	for _, is := range fake.Issues() {
		if _, ok := mirror.LoadBase(ctx.MirrorDir, is.Identity.Key); !ok {
			snap := *is
			if snap.Context.Status == "" {
				snap.Context.Status = snap.Status
			}
			_ = mirror.SaveBase(ctx.MirrorDir, &snap)
		}
	}
}
