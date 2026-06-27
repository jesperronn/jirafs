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

// TestRunPlan_NoArgs verifies that calling RunPlan with no arguments
// returns exit code 0 and lists local issues (possibly empty).
func TestRunPlan_NoArgs(t *testing.T) {
	exit := RunPlan([]string{})
	if exit != 0 {
		t.Errorf("RunPlan([]) = %d, want 0", exit)
	}
}

// TestRunPlan_UnknownSubcommand verifies that RunPlan with an unknown
// subcommand (not "help") returns exit code 1.
func TestRunPlan_UnknownSubcommand(t *testing.T) {
	exit := RunPlan([]string{"bogus"})
	if exit != 1 {
		t.Errorf("RunPlan([\"bogus\"]) = %d, want 1", exit)
	}
}

// TestRunPlan_NoKey_NoProject verifies that plan without an issue key
// and without a resolvable project returns exit code 1.
func TestRunPlan_NoKey_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPlanTestIO(t)
	exit := RunPlan([]string{"--cwd", filepath.Join(tmpDir, "outside")})
	if exit != 1 {
		t.Fatalf("RunPlan([\"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestRunPlan_NoKey_NoLocalDirs verifies that plan without an issue key
// but with a project that has no local_dirs returns exit code 0.
func TestRunPlan_NoKey_NoLocalDirs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create settings with a project that has no local_dirs.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
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

	stdout, stderr := withPlanTestIO(t)
	exit := RunPlan([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "no local directories configured") {
		t.Fatalf("stdout = %q, want no local directories configured", output)
	}
}

// TestRunPlan_NoKey_NoIssueFiles verifies that plan without an issue key
// but with local_dirs that contain no issue files returns exit code 0.
func TestRunPlan_NoKey_NoIssueFiles(t *testing.T) {
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

	stdout, stderr := withPlanTestIO(t)
	exit := RunPlan([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "no issue files found") {
		t.Fatalf("stdout = %q, want no issue files found", output)
	}
}

// TestRunPlan_NoKey_ListIssues verifies that plan without an issue key
// lists all local issues with their planned operations.
func TestRunPlan_NoKey_ListIssues(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create local issue files.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	issue1Content := `---
key: PROJ-1
type: story
project:
  type: project
  value: PROJ
summary: First issue
---

Summary: First issue
`
	issue2Content := `---
key: PROJ-2
type: story
project:
  type: project
  value: PROJ
summary: Second issue
---

Summary: Second issue
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-1.md"), []byte(issue1Content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-2.md"), []byte(issue2Content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
	fake.SetIssue("PROJ-1", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-1",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary: "First issue",
	})
	fake.SetIssue("PROJ-2", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-2",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary: "Second issue",
	})
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "PROJ-1") {
		t.Fatalf("stdout = %q, want PROJ-1", output)
	}
	if !strings.Contains(output, "PROJ-2") {
		t.Fatalf("stdout = %q, want PROJ-2", output)
	}
	if !strings.Contains(output, "no changes needed") {
		t.Fatalf("stdout = %q, want no changes needed", output)
	}
	if !strings.Contains(output, "2 issue(s)") {
		t.Fatalf("stdout = %q, want 2 issue(s)", output)
	}
}

// TestRunPlan_NoKey_ListIssues_WithChanges verifies that plan without an
// issue key lists issues with planned operations when changes exist.
func TestRunPlan_NoKey_ListIssues_WithChanges(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a local issue with a different summary.
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
summary: Local modified
---

Summary: Local modified
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-1.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
	fake.SetIssue("PROJ-1", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-1",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary: "Remote original",
	})
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "PROJ-1") {
		t.Fatalf("stdout = %q, want PROJ-1", output)
	}
	if !strings.Contains(output, "1 operation(s)") {
		t.Fatalf("stdout = %q, want 1 operation(s)", output)
	}
	if !strings.Contains(output, "summary") {
		t.Fatalf("stdout = %q, want summary field", output)
	}
}

// TestRunPlan_NoKey_SkipsUnparseable verifies that plan without an issue
// key skips files that cannot be parsed as issues.
func TestRunPlan_NoKey_SkipsUnparseable(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a parseable issue and a non-issue file.
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
summary: Valid issue
---

Summary: Valid issue
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-1.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Non-issue .md file.
	if err := os.WriteFile(filepath.Join(localDir, "README.md"), []byte("not an issue"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
	fake.SetIssue("PROJ-1", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-1",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary: "Valid issue",
	})
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "PROJ-1") {
		t.Fatalf("stdout = %q, want PROJ-1", output)
	}
	// README.md should be skipped (not an issue).
	if strings.Contains(output, "README") {
		t.Fatalf("stdout = %q, should not contain README", output)
	}
}

// TestRunPlan_Help verifies that "help" returns exit code 0.
func TestRunPlan_Help(t *testing.T) {
	exit := RunPlan([]string{"help"})
	if exit != 0 {
		t.Errorf("RunPlan([\"help\"]) = %d, want 0", exit)
	}
}

func withPlanTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := planStdout
	oldStderr := planStderr
	planStdout = stdout
	planStderr = stderr
	t.Cleanup(func() {
		planStdout = oldStdout
		planStderr = oldStderr
	})
	return stdout, stderr
}

func withPlanClientFactory(t *testing.T, factory func(*config.Settings, *context.Context, string) (jira.Client, error)) {
	t.Helper()
	oldFactory := planClientFactory
	planClientFactory = factory
	t.Cleanup(func() {
		planClientFactory = oldFactory
	})
}



// TestRunPlan_NoKey_ProjectNotFound verifies that plan without an issue key
// returns exit code 1 when --project points to a non-existent project.
func TestRunPlan_NoKey_ProjectNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a local directory with an issue file.
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
summary: Test issue
---

Summary: Test issue
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-1.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPlanTestIO(t)
	exit := RunPlan([]string{"--project", "nonexistent"})
	if exit != 1 {
		t.Fatalf("RunPlan([\"--project\", \"nonexistent\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in settings") {
		t.Fatalf("stderr = %q, want not found in settings", stderr.String())
	}
}

// TestRunPlan_NoKey_ClientError verifies that plan without an issue key
// skips issues that fail client creation.
func TestRunPlan_NoKey_ClientError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a local directory with an issue file.
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
summary: Test issue
---

Summary: Test issue
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-1.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Return an error from the client factory.
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, os.ErrPermission
	})
	_, stderr := withPlanTestIO(t)
	exit := RunPlan([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"--project\", \"test\"]) = %d, want 0", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create client") {
		t.Fatalf("stderr = %q, want client creation error", stderr.String())
	}
}

// TestRunPlan_NoKey_NoParseableFiles verifies that plan without an issue key
// reports no parseable files when all files are unparseable.
func TestRunPlan_NoKey_NoParseableFiles(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a local directory with only unparseable .md files.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "README.md"), []byte("just a readme"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-1.md"), []byte("not an issue file"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	stdout, stderr := withPlanTestIO(t)
	exit := RunPlan([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "no parseable issue files found") {
		t.Fatalf("stdout = %q, want no parseable issue files found", output)
	}
}

// TestRunPlan_TooManyArgs verifies that plan with too many arguments
// returns exit code 1.
func TestRunPlan_TooManyArgs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPlanTestIO(t)
	exit := RunPlan([]string{"--project", "test", "PROJ-1", "extra"})
	if exit != 1 {
		t.Fatalf("RunPlan([\"PROJ-1\", \"extra\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "too many positional arguments") {
		t.Fatalf("stderr = %q, want too many positional arguments", stderr.String())
	}
}

// TestRunPlan_NoProjectResolved verifies that plan without a resolvable
// project returns exit code 1.
func TestRunPlan_NoProjectResolved(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPlanTestIO(t)
	exit := RunPlan([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPlan([\"--project\", \"test\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestRunPlan_NoLocalIssue verifies that plan returns an error when
// the local issue file is not found.
func TestRunPlan_NoLocalIssue(t *testing.T) {
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
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPlan([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in local directories") {
		t.Fatalf("stderr = %q, want not found in local directories", stderr.String())
	}
}

// TestRunPlan_Success_NoChanges verifies that plan reports no changes
// when local and remote match.
func TestRunPlan_Success_NoChanges(t *testing.T) {
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
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "no changes needed") {
		t.Fatalf("stdout = %q, want \"no changes needed\"", output)
	}
	if !strings.Contains(output, "PROJ-42") {
		t.Fatalf("stdout = %q, want PROJ-42 key", output)
	}
}

// TestRunPlan_Success_WithChanges verifies that plan reports operations
// when local and remote differ.
func TestRunPlan_Success_WithChanges(t *testing.T) {
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
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
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
}

// TestRunPlan_Success_MultipleOperations verifies that plan reports
// multiple operations when many fields differ.
func TestRunPlan_Success_MultipleOperations(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Create a local issue with multiple field changes.
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
labels:
  - bug
  - priority
assignee: jdoe
status: In Progress
sprint: Sprint 42
fix_versions:
  - 1.0
  - 2.0
---

Summary: Local summary
Local description body
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-42.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	assignee := "jsmith"
	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-42",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary:     "Remote summary",
		Description: "Remote description body",
		Labels:      []string{"enhancement"},
		Assignee:    &assignee,
		Status:      "To Do",
		Sprint:      "Sprint 41",
		FixVersions: []string{"1.0"},
	})
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "7 operation(s)") {
		t.Fatalf("stdout = %q, want 7 operation(s)", output)
	}
	if !strings.Contains(output, "PROJ-42") {
		t.Fatalf("stdout = %q, want PROJ-42 key", output)
	}
}

// TestRunPlan_FetchError verifies that plan returns an error when
// the Jira fetch fails.
func TestRunPlan_FetchError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetErr("fetch", jira.NewHTTPErr(500, "server error"))
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPlan([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "server error") {
		t.Fatalf("stderr = %q, want server error", stderr.String())
	}
}

// TestRunPlan_NotFound verifies that plan returns a not found error
// when the remote issue does not exist.
func TestRunPlan_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test", "PROJ-99"})
	if exit != 1 {
		t.Fatalf("RunPlan([\"PROJ-99\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not_found") {
		t.Fatalf("stderr = %q, want not_found error", stderr.String())
	}
}

// TestRunPlan_ClientCreationError verifies that plan returns an error
// when the Jira client cannot be created.
func TestRunPlan_ClientCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, os.ErrPermission
	})
	_, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPlan([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create Jira client") {
		t.Fatalf("stderr = %q, want client creation error", stderr.String())
	}
}

// TestRunPlan_LocalIssueUnparseable verifies that plan skips files that
// cannot be parsed as issues.
func TestRunPlan_LocalIssueUnparseable(t *testing.T) {
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
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPlanTestIO(t)

	exit := RunPlan([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPlan([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in local directories") {
		t.Fatalf("stderr = %q, want not found in local directories", stderr.String())
	}
}

// TestBuildPlanClient_Success verifies that buildPlanClient
// creates a valid Jira client from settings.
func TestBuildPlanClient_Success(t *testing.T) {
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

	client, err := buildPlanClient(settings, &context.Context{Instance: "default"}, ".")
	if err != nil {
		t.Fatalf("buildPlanClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestRunPlan_ResolveContextError verifies that plan returns an error
// when no project can be resolved for the given cwd.
func TestRunPlan_ResolveContextError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPlanTestIO(t)
	exit := RunPlan([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunPlan([\"--project\", \"test\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestResolvePlanContext_NoProject verifies that resolvePlanContext
// returns false when no project can be resolved for the given cwd.
func TestResolvePlanContext_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	_, stderr := withPlanTestIO(t)

	ctx, ok := resolvePlanContext(settings, "", filepath.Join(tmpDir, "outside"))
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

// TestBuildPlanClient_CredentialError verifies that buildPlanClient
// returns an error when the instance has no credentials configured.
func TestBuildPlanClient_CredentialError(t *testing.T) {
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

	_, err := buildPlanClient(settings, &context.Context{Instance: "default"}, ".")
	if err == nil {
		t.Fatal("expected error for instance with no credentials")
	}
	if !strings.Contains(err.Error(), "no_usable_instance") {
		t.Fatalf("error = %q, want no_usable_instance", err.Error())
	}

}
