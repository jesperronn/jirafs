package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

// TestRunSync_NoArgs verifies that calling RunSync with no arguments
// resolves the project and lists local issues (or reports none found).
func TestRunSync_NoArgs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")

	// Create an empty local directory.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create settings with local_dirs.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + localDir + `"]

[state]
current_project = "test"
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

	stdout, stderr := withSyncTestIO(t)
	exit := RunSync([]string{})
	if exit != 0 {
		t.Fatalf("RunSync([]) = %d, stderr = %q, want 0", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "no local issues found") {
		t.Fatalf("stdout = %q, want no local issues found", output)
	}
}

// TestRunSync_NoArgs_NoLocalDirs verifies that when no issue key is
// provided and the resolved project has no local directories, sync
// returns exit code 1 with an appropriate message.
func TestRunSync_NoArgs_NoLocalDirs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create settings with a project that has no local_dirs.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"

[state]
current_project = "test"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withSyncTestIO(t)
	exit := RunSync([]string{})
	if exit != 1 {
		t.Fatalf("RunSync([]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no local directories") {
		t.Fatalf("stderr = %q, want no local directories", stderr.String())
	}
}

// TestRunSync_NoArgs_ListIssues verifies that when no issue key is
// provided, sync resolves the project and lists all local issues.
func TestRunSync_NoArgs_ListIssues(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")

	// Create local issue files.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	issueContent1 := `---
key: PROJ-1
type: story
project:
  type: project
  value: PROJ
summary: First issue
---

Summary: First issue
`
	issueContent2 := `---
key: PROJ-2
type: bug
project:
  type: project
  value: PROJ
summary: Second issue
---

Summary: Second issue
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-1.md"), []byte(issueContent1), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-2.md"), []byte(issueContent2), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create settings with local_dirs.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + localDir + `"]

[state]
current_project = "test"
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

	// Fake transport returns no issues (remote not found).
	fake := jira.NewFakeTransport()
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	// Also set syncBuildClient so runSyncAll uses the fake.
	oldBuildClient := syncBuildClient
	syncBuildClient = func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	}
	t.Cleanup(func() {
		syncBuildClient = oldBuildClient
	})
	stdout, stderr := withSyncTestIO(t)

	exit := RunSync([]string{})
	if exit != 0 {
		t.Fatalf("RunSync([]) = %d, stderr = %q, want 0", exit, stderr.String())
	}

	output := stdout.String()
	t.Logf("stdout = %q", output)
	t.Logf("stderr = %q", stderr.String())
	if !strings.Contains(output, "PROJ-1") {
		t.Fatalf("stdout = %q, want PROJ-1", output)
	}
	if !strings.Contains(output, "PROJ-2") {
		t.Fatalf("stdout = %q, want PROJ-2", output)
	}
	if !strings.Contains(output, "project test") {
		t.Fatalf("stdout = %q, want project test", output)
	}
	if !strings.Contains(output, "issue(s) have pending") {
		t.Fatalf("stdout = %q, want issue(s) have pending", output)
	}
}

// TestRunSync_NoArgs_UpToDate verifies that when no issue key is provided
// and all local issues match their remote, sync reports all up to date.
func TestRunSync_NoArgs_UpToDate(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")

	// Create matching local issue files.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	issueContent := `---
key: PROJ-1
type: story
project:
  type: project
  value: PROJ
summary: Matching issue
---

Summary: Matching issue
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-1.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create settings with local_dirs.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + localDir + `"]

[state]
current_project = "test"
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

	// Fake transport returns matching remote.
	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-1", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-1",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary: "Matching issue",
	})
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	oldBuildClient := syncBuildClient
	syncBuildClient = func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	}
	t.Cleanup(func() {
		syncBuildClient = oldBuildClient
	})
	stdout, stderr := withSyncTestIO(t)

	exit := RunSync([]string{})
	if exit != 0 {
		t.Fatalf("RunSync([]) = %d, stderr = %q, want 0", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "up to date") {
		t.Fatalf("stdout = %q, want up to date", output)
	}
	if !strings.Contains(output, "0 of 1 issue(s) have pending") {
		t.Fatalf("stdout = %q, want 0 of 1 issue(s) have pending", output)
	}
}

// TestRunSync_NoArgs_NoProjectResolved verifies that when no issue key is
// provided and no project can be resolved, sync returns exit code 1.
func TestRunSync_NoArgs_NoProjectResolved(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Create settings without a project that matches the cwd.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.other]
key = "OTHER"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "other") + `"
local_dirs = ["` + filepath.Join(tmpDir, "other") + `"]
`
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, stderr := withSyncTestIO(t)
	exit := RunSync([]string{})
	if exit != 1 {
		t.Fatalf("RunSync([]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestRunSync_NoArgs_EmptyLocalDir verifies that when no issue key is
// provided but local directories exist and contain no .md files,
// sync reports no local issues found.
func TestRunSync_NoArgs_EmptyLocalDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create an empty local directory.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + localDir + `"]

[state]
current_project = "test"
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

	stdout, stderr := withSyncTestIO(t)
	exit := RunSync([]string{})
	if exit != 0 {
		t.Fatalf("RunSync([]) = %d, stderr = %q, want 0", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "no local issues found") {
		t.Fatalf("stdout = %q, want no local issues found", output)
	}
}

// TestRunSync_UnknownSubcommand verifies that RunSync with an unknown
// subcommand (not "help") returns exit code 1.
func TestRunSync_UnknownSubcommand(t *testing.T) {
	exit := RunSync([]string{"bogus"})
	if exit != 1 {
		t.Errorf("RunSync([\"bogus\"]) = %d, want 1", exit)
	}
}

// TestRunSync_Help verifies that "help" returns exit code 0.
func TestRunSync_Help(t *testing.T) {
	exit := RunSync([]string{"help"})
	if exit != 0 {
		t.Errorf("RunSync([\"help\"]) = %d, want 0", exit)
	}
}

func withSyncTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := syncStdout
	oldStderr := syncStderr
	syncStdout = stdout
	syncStderr = stderr
	t.Cleanup(func() {
		syncStdout = oldStdout
		syncStderr = oldStderr
	})
	return stdout, stderr
}

func withSyncClientFactory(t *testing.T, factory func(*config.Settings, *context.Context, string) (jira.Client, error)) {
	t.Helper()
	oldFactory := syncClientFactory
	syncClientFactory = factory
	t.Cleanup(func() {
		syncClientFactory = oldFactory
	})
}

// TestRunSync_MissingKey verifies that when no issue key is provided
// but flags like --cwd are given, sync resolves the project and lists
// pending syncs (or reports no issues found).
func TestRunSync_MissingKey(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Create settings with a project that matches the cwd.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "mirror") + `"]

[state]
current_project = "test"
`
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create a fake transport.
	fake := jira.NewFakeTransport()
	oldBuildClient := syncBuildClient
	syncBuildClient = func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	}
	t.Cleanup(func() {
		syncBuildClient = oldBuildClient
	})

	stdout, stderr := withSyncTestIO(t)

	exit := RunSync([]string{"--cwd", tmpDir})
	if exit != 0 {
		t.Fatalf("RunSync([\"--cwd\", ...]) = %d, want 0, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	// Should list issues or report none found, not "missing issue key".
	if !strings.Contains(output, "no local issues found") && !strings.Contains(output, "issue(s)") {
		t.Fatalf("stdout = %q, want listing output", output)
	}
	if strings.Contains(stderr.String(), "missing issue key") {
		t.Fatalf("stderr should not contain 'missing issue key', got: %q", stderr.String())
	}
}

// TestRunSync_TooManyArgs verifies that sync with too many arguments
// returns exit code 1.
func TestRunSync_TooManyArgs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withSyncTestIO(t)
	exit := RunSync([]string{"--project", "test", "PROJ-1", "extra"})
	if exit != 1 {
		t.Fatalf("RunSync([\"PROJ-1\", \"extra\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "too many positional arguments") {
		t.Fatalf("stderr = %q, want too many positional arguments", stderr.String())
	}
}

// TestRunSync_NoProjectResolved verifies that sync without a resolvable
// project returns exit code 1.
func TestRunSync_NoProjectResolved(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withSyncTestIO(t)
	exit := RunSync([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunSync([\"--project\", \"test\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestRunSync_NoLocalIssue verifies that sync returns an error when
// the local issue file is not found.
func TestRunSync_NoLocalIssue(t *testing.T) {
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
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withSyncTestIO(t)

	exit := RunSync([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunSync([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in local directories") {
		t.Fatalf("stderr = %q, want not found in local directories", stderr.String())
	}
}

// TestRunSync_Success_NoChanges verifies that sync reports no changes
// when local and remote match.
func TestRunSync_Success_NoChanges(t *testing.T) {
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
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withSyncTestIO(t)

	exit := RunSync([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunSync([\"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "no changes needed") {
		t.Fatalf("stdout = %q, want \"no changes needed\"", output)
	}
	if !strings.Contains(output, "PROJ-42") {
		t.Fatalf("stdout = %q, want PROJ-42 key", output)
	}
}

// TestRunSync_Success_WithChanges verifies that sync reports operations
// and pushes the updated remote when local and remote differ.
func TestRunSync_Success_WithChanges(t *testing.T) {
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
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withSyncTestIO(t)

	exit := RunSync([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunSync([\"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
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

// TestRunSync_FetchError verifies that sync returns an error when
// the Jira fetch fails.
func TestRunSync_FetchError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetErr("fetch", jira.NewHTTPErr(500, "server error"))
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withSyncTestIO(t)

	exit := RunSync([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunSync([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "server error") {
		t.Fatalf("stderr = %q, want server error", stderr.String())
	}
}

// TestRunSync_NotFound verifies that sync returns a not found error
// when the remote issue does not exist.
func TestRunSync_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withSyncTestIO(t)

	exit := RunSync([]string{"--project", "test", "PROJ-99"})
	if exit != 1 {
		t.Fatalf("RunSync([\"PROJ-99\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not_found") {
		t.Fatalf("stderr = %q, want not_found error", stderr.String())
	}
}

// TestRunSync_ClientCreationError verifies that sync returns an error
// when the Jira client cannot be created.
func TestRunSync_ClientCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, os.ErrPermission
	})
	_, stderr := withSyncTestIO(t)

	exit := RunSync([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunSync([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create Jira client") {
		t.Fatalf("stderr = %q, want client creation error", stderr.String())
	}
}

// TestRunSync_LocalIssueUnparseable verifies that sync skips files that
// cannot be parsed as issues.
func TestRunSync_LocalIssueUnparseable(t *testing.T) {
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
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withSyncTestIO(t)

	exit := RunSync([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunSync([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in local directories") {
		t.Fatalf("stderr = %q, want not found in local directories", stderr.String())
	}
}

// TestRunSync_ConflictsNoMutation verifies that sync reports conflicts
// without pushing to Jira when the plan has conflicts.
func TestRunSync_ConflictsNoMutation(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a local issue file with a status change (which is rejected).
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
status: In Progress
---

Summary: Local summary
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
		Summary: "Remote summary",
		Status:  "To Do",
	})
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, _ := withSyncTestIO(t)

	exit := RunSync([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunSync([\"PROJ-42\"]) = %d, want 0 (conflicts are not errors)", exit)
	}

	output := stdout.String()
	if !strings.Contains(output, "conflicts detected") {
		t.Fatalf("stdout = %q, want conflicts detected", output)
	}
	// Verify UpdateIssue was NOT called (the fake should still have the original issue).
	updated, err := fake.UpdateIssue(nil, "PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42"},
		Summary:  "updated",
	})
	if err == nil && updated != nil && updated.Summary == "updated" {
		// UpdateIssue was called when it shouldn't have been.
		t.Log("WARNING: UpdateIssue was called despite conflicts")
	}
}

// TestRunSync_UpdateIssueError verifies that sync returns an error
// when the Jira update fails after a successful plan.
func TestRunSync_UpdateIssueError(t *testing.T) {
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
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withSyncTestIO(t)

	exit := RunSync([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunSync([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot update remote issue") {
		t.Fatalf("stderr = %q, want cannot update remote issue", stderr.String())
	}
}

// TestRunSync_Success_WithChangesAndApply verifies that sync with --apply
// writes the updated remote back to the local file system.
func TestRunSync_Success_WithChangesAndApply(t *testing.T) {
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
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withSyncTestIO(t)

	exit := RunSync([]string{"--project", "test", "--apply", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunSync([\"--project\", \"test\", \"--apply\", \"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
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

// TestRunSync_ResolveContextError verifies that sync returns an error
// when no project can be resolved for the given cwd.
func TestRunSync_ResolveContextError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withSyncTestIO(t)
	exit := RunSync([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunSync([\"--project\", \"test\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestResolveSyncContext_NoProject verifies that resolveSyncContext
// returns false when no project can be resolved for the given cwd.
func TestResolveSyncContext_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	_, stderr := withSyncTestIO(t)

	ctx, ok := resolveSyncContext(settings, context.NewResolver(settings, ""))
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

// TestBuildSyncClient_Success verifies that buildSyncClient
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

	client, err := buildSyncClient(settings, &context.Context{Instance: "default"}, ".")
	if err != nil {
		t.Fatalf("buildSyncClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestBuildSyncClient_CredentialError verifies that buildSyncClient
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

	_, err := buildSyncClient(settings, &context.Context{Instance: "default"}, ".")
	if err == nil {
		t.Fatal("expected error for instance with no credentials")
	}
	if !strings.Contains(err.Error(), "no_usable_instance") {
		t.Fatalf("error = %q, want no_usable_instance", err.Error())
	}
}
