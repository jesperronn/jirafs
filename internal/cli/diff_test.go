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

// TestRunDiff_NoArgs verifies that calling RunDiff with no arguments
// returns exit code 0 and lists local issues (possibly empty).
func TestRunDiff_NoArgs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create settings with a default current_project, so RunDiff doesn't need --project flag.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]

[State]
current_project = "test"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	stdout, _ := withDiffTestIO(t)
	exit := RunDiff([]string{})
	if exit != 0 {
		t.Errorf("RunDiff([]) = %d, want 0", exit)
	}
	// Verify it found no issue files (since tmpDir/local doesn't exist).
	if !strings.Contains(stdout.String(), "no issue files found") {
		t.Errorf("stdout = %q, want 'no issue files found'", stdout.String())
	}
}

// TestRunDiff_UnknownSubcommand verifies that RunDiff with an unknown
// subcommand (not "help") returns exit code 1.
func TestRunDiff_UnknownSubcommand(t *testing.T) {
	exit := RunDiff([]string{"bogus"})
	if exit != 1 {
		t.Errorf("RunDiff([\"bogus\"]) = %d, want 1", exit)
	}
}

// TestRunDiff_NoKey_NoProject verifies that diff without an issue key
// and without a resolvable project returns exit code 1.
func TestRunDiff_NoKey_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withDiffTestIO(t)
	exit := RunDiff([]string{"--cwd", filepath.Join(tmpDir, "outside")})
	if exit != 1 {
		t.Fatalf("RunDiff([\"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestRunDiff_NoKey_NoLocalDirs verifies that diff without an issue key
// but with a project that has no local_dirs returns exit code 0.
func TestRunDiff_NoKey_NoLocalDirs(t *testing.T) {
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

	stdout, stderr := withDiffTestIO(t)
	exit := RunDiff([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunDiff([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "no local directories configured") {
		t.Fatalf("stdout = %q, want no local directories configured", output)
	}
}

// TestRunDiff_NoKey_NoIssueFiles verifies that diff without an issue key
// but with local_dirs that contain no issue files returns exit code 0.
func TestRunDiff_NoKey_NoIssueFiles(t *testing.T) {
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

	stdout, stderr := withDiffTestIO(t)
	exit := RunDiff([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunDiff([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "no issue files found") {
		t.Fatalf("stdout = %q, want no issue files found", output)
	}
}

// TestRunDiff_NoKey_ListIssues verifies that diff without an issue key
// lists all local issues with their planned operations.
func TestRunDiff_NoKey_ListIssues(t *testing.T) {
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
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunDiff([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
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

// TestRunDiff_NoKey_ListIssues_UsesSelectedProject verifies that calling
// RunDiff without an issue key and without --project uses the remembered
// (selected) project from settings state. This is the core acceptance
// criterion for B095b: "jirafs diff uses the selected project when the
// issue key is omitted."
func TestRunDiff_NoKey_ListIssues_UsesSelectedProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create local issue files.
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
summary: First issue
---

Summary: First issue
`
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-1.md"), []byte(issueContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create settings with a project and a remembered current project.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + localDir + `"]

[State]
current_project = "test"
`
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
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withDiffTestIO(t)

	// Call RunDiff with NO issue key and NO --project flag.
	// It should use the remembered project "test" from settings state.
	exit := RunDiff([]string{})
	if exit != 0 {
		t.Fatalf("RunDiff([]) = %d, stderr = %q, want 0", exit, stderr.String())
	}

	output := stdout.String()
	// Verify it found and listed the issue from the remembered project.
	if !strings.Contains(output, "PROJ-1") {
		t.Fatalf("stdout = %q, want PROJ-1", output)
	}
	if !strings.Contains(output, "no changes needed") {
		t.Fatalf("stdout = %q, want no changes needed", output)
	}
	if !strings.Contains(output, "1 issue(s)") {
		t.Fatalf("stdout = %q, want 1 issue(s)", output)
	}
}

// TestRunDiff_NoKey_ListIssues_WithChanges verifies that diff without an
// issue key lists issues with planned operations when changes exist.
func TestRunDiff_NoKey_ListIssues_WithChanges(t *testing.T) {
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
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunDiff([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
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

// TestRunDiff_NoKey_SkipsUnparseable verifies that diff without an issue
// key skips files that cannot be parsed as issues.
func TestRunDiff_NoKey_SkipsUnparseable(t *testing.T) {
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
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunDiff([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
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

// TestRunDiff_Help verifies that "help" returns exit code 0.
func TestRunDiff_Help(t *testing.T) {
	exit := RunDiff([]string{"help"})
	if exit != 0 {
		t.Errorf("RunDiff([\"help\"]) = %d, want 0", exit)
	}
}

func withDiffTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := diffStdout
	oldStderr := diffStderr
	diffStdout = stdout
	diffStderr = stderr
	t.Cleanup(func() {
		diffStdout = oldStdout
		diffStderr = oldStderr
	})
	return stdout, stderr
}

func withDiffClientFactory(t *testing.T, factory func(*config.Settings, *context.Context, string) (jira.Client, error)) {
	t.Helper()
	oldFactory := diffClientFactory
	diffClientFactory = factory
	t.Cleanup(func() {
		diffClientFactory = oldFactory
	})
}

// TestRunDiff_NoKey_ProjectNotFound verifies that diff without an issue key
// returns exit code 1 when --project points to a non-existent project.
func TestRunDiff_NoKey_ProjectNotFound(t *testing.T) {
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

	_, stderr := withDiffTestIO(t)
	exit := RunDiff([]string{"--project", "nonexistent"})
	if exit != 1 {
		t.Fatalf("RunDiff([\"--project\", \"nonexistent\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in settings") {
		t.Fatalf("stderr = %q, want not found in settings", stderr.String())
	}
}

// TestRunDiff_NoKey_ClientError verifies that diff without an issue key
// skips issues that fail client creation.
func TestRunDiff_NoKey_ClientError(t *testing.T) {
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
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, os.ErrPermission
	})
	_, stderr := withDiffTestIO(t)
	exit := RunDiff([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunDiff([\"--project\", \"test\"]) = %d, want 0", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create client") {
		t.Fatalf("stderr = %q, want client creation error", stderr.String())
	}
}

// TestRunDiff_NoKey_NoParseableFiles verifies that diff without an issue key
// reports no parseable files when all files are unparseable.
func TestRunDiff_NoKey_NoParseableFiles(t *testing.T) {
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

	stdout, stderr := withDiffTestIO(t)
	exit := RunDiff([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunDiff([\"--project\", \"test\"]) = %d, stderr = %q, want 0", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "no parseable issue files found") {
		t.Fatalf("stdout = %q, want no parseable issue files found", output)
	}
}

// TestRunDiff_NoProjectResolved verifies that diff without a resolvable
// project returns exit code 1.
func TestRunDiff_NoProjectResolved(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withDiffTestIO(t)
	exit := RunDiff([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunDiff([\"--project\", \"test\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestRunDiff_NoLocalIssue verifies that diff returns an error when
// the local issue file is not found.
func TestRunDiff_NoLocalIssue(t *testing.T) {
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
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunDiff([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in local directories") {
		t.Fatalf("stderr = %q, want not found in local directories", stderr.String())
	}
}

// TestRunDiff_Success_NoChanges verifies that diff reports no changes
// when local and remote match.
func TestRunDiff_Success_NoChanges(t *testing.T) {
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
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunDiff([\"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "no changes needed") {
		t.Fatalf("stdout = %q, want \"no changes needed\"", output)
	}
	if !strings.Contains(output, "PROJ-42") {
		t.Fatalf("stdout = %q, want PROJ-42 key", output)
	}
}

// TestRunDiff_Success_WithChanges verifies that diff reports operations
// when local and remote differ.
func TestRunDiff_Success_WithChanges(t *testing.T) {
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
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunDiff([\"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
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

// TestRunDiff_Success_MultipleOperations verifies that diff reports
// multiple operations when many fields differ.
func TestRunDiff_Success_MultipleOperations(t *testing.T) {
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
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunDiff([\"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "7 operation(s)") {
		t.Fatalf("stdout = %q, want 7 operation(s)", output)
	}
	if !strings.Contains(output, "PROJ-42") {
		t.Fatalf("stdout = %q, want PROJ-42 key", output)
	}
}

// TestRunDiff_FetchError verifies that diff returns an error when
// the Jira fetch fails.
func TestRunDiff_FetchError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetErr("fetch", jira.NewHTTPErr(500, "server error"))
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunDiff([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "server error") {
		t.Fatalf("stderr = %q, want server error", stderr.String())
	}
}

// TestRunDiff_NotFound verifies that diff returns a not found error
// when the remote issue does not exist.
func TestRunDiff_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test", "PROJ-99"})
	if exit != 1 {
		t.Fatalf("RunDiff([\"PROJ-99\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not_found") {
		t.Fatalf("stderr = %q, want not_found error", stderr.String())
	}
}

// TestRunDiff_ClientCreationError verifies that diff returns an error
// when the Jira client cannot be created.
func TestRunDiff_ClientCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, os.ErrPermission
	})
	_, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunDiff([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create Jira client") {
		t.Fatalf("stderr = %q, want client creation error", stderr.String())
	}
}

// TestRunDiff_LocalIssueUnparseable verifies that diff skips files that
// cannot be parsed as issues.
func TestRunDiff_LocalIssueUnparseable(t *testing.T) {
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
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withDiffTestIO(t)

	exit := RunDiff([]string{"--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunDiff([\"PROJ-42\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in local directories") {
		t.Fatalf("stderr = %q, want not found in local directories", stderr.String())
	}
}

// TestBuildPlanClient_Success verifies that newJiraClient
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

	client, err := newJiraClient(settings, &context.Context{Instance: "default"}, ".")
	if err != nil {
		t.Fatalf("newJiraClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestRunDiff_ResolveContextError verifies that diff returns an error
// when no project can be resolved for the given cwd.
func TestRunDiff_ResolveContextError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withDiffTestIO(t)
	exit := RunDiff([]string{"--cwd", filepath.Join(tmpDir, "outside"), "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunDiff([\"--project\", \"test\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestResolveDiffContext_NoProject verifies that resolveDiffContext
// returns false when no project can be resolved for the given cwd.
func TestResolveDiffContext_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	_, stderr := withDiffTestIO(t)

	ctx, ok := resolveDiffContext(settings, "", filepath.Join(tmpDir, "outside"))
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

// TestBuildPlanClient_CredentialError verifies that newJiraClient
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

	_, err := newJiraClient(settings, &context.Context{Instance: "default"}, ".")
	if err == nil {
		t.Fatal("expected error for instance with no credentials")
	}
	if !strings.Contains(err.Error(), "no_usable_instance") {
		t.Fatalf("error = %q, want no_usable_instance", err.Error())
	}

}

// diffFixture writes settings with one local dir holding the given issue
// files (key -> summary) and returns a fake Jira holding remote summaries.
func diffFixture(t *testing.T, local, remote map[string]string) {
	t.Helper()
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := jira.NewFakeTransport()
	for key, sum := range local {
		doc := "---\nkey: " + key + "\ntype: story\nproject:\n  type: project\n  value: PROJ\nsummary: " + sum + "\n---\n\nSummary: " + sum + "\n"
		if err := os.WriteFile(filepath.Join(localDir, key+".md"), []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		fake.SetIssue(key, &schema.Issue{
			Identity: schema.IssueIdentity{Key: schema.IssueKey(key), Type: "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"}},
			Summary: remote[key],
		})
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
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)
	withDiffClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
}

func TestRunDiff_NoArgs_TwoModifiedFiles(t *testing.T) {
	diffFixture(t,
		map[string]string{"PROJ-1": "L1", "PROJ-2": "L2"},
		map[string]string{"PROJ-1": "R1", "PROJ-2": "R2"})
	stdout, stderr := withDiffTestIO(t)
	if exit := RunDiff([]string{"--project", "test"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"PROJ-1: 1 operation(s)", "PROJ-2: 1 operation(s)", "2 issue(s) changed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %q, want %q", out, want)
		}
	}
}

func TestRunDiff_ExitCode(t *testing.T) {
	diffFixture(t,
		map[string]string{"PROJ-1": "L1", "PROJ-2": "same"},
		map[string]string{"PROJ-1": "R1", "PROJ-2": "same"})
	stdout, _ := withDiffTestIO(t)
	if exit := RunDiff([]string{"--project", "test", "--exit-code", "PROJ-1"}); exit != 1 {
		t.Fatalf("with changes: exit = %d, want 1", exit)
	}
	if !strings.Contains(stdout.String(), "1 issue(s) changed") {
		t.Fatalf("stdout = %q, want summary", stdout.String())
	}
	if exit := RunDiff([]string{"--project", "test", "--exit-code", "PROJ-2"}); exit != 0 {
		t.Fatalf("without changes: exit = %d, want 0", exit)
	}
	if exit := RunDiff([]string{"--project", "test", "PROJ-1"}); exit != 0 {
		t.Fatalf("without --exit-code: exit = %d, want 0", exit)
	}
	if exit := RunDiff([]string{"--project", "test", "--exit-code"}); exit != 1 {
		t.Fatalf("no-arg with changes: exit = %d, want 1", exit)
	}
}

func TestRunDiff_MultipleKeys(t *testing.T) {
	diffFixture(t,
		map[string]string{"PROJ-1": "L1", "PROJ-2": "same"},
		map[string]string{"PROJ-1": "R1", "PROJ-2": "same"})
	stdout, stderr := withDiffTestIO(t)
	if exit := RunDiff([]string{"--project", "test", "PROJ-1", "PROJ-2"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "1 operation(s) for issue PROJ-1") || !strings.Contains(out, "issue PROJ-2") || !strings.Contains(out, "1 issue(s) changed") {
		t.Fatalf("stdout = %q", out)
	}
	if exit := RunDiff([]string{"--project", "test", "PROJ-1", "PROJ-404"}); exit != 1 {
		t.Fatalf("missing key: exit = %d, want 1", exit)
	}
}

func TestRunDiff_SkipsFilesUnchangedSinceSync(t *testing.T) {
	diffFixture(t, map[string]string{"PROJ-1": "L1"}, map[string]string{"PROJ-1": "R1"})
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(settings.Projects["test"].LocalDirs[0], "PROJ-1.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	synced := schema.StampSyncedHash(string(data))
	if err := os.WriteFile(path, []byte(synced), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _ := withDiffTestIO(t)
	if exit := RunDiff([]string{"--project", "test", "--exit-code"}); exit != 0 {
		t.Fatalf("exit = %d, want 0 (nothing changed locally)", exit)
	}
	if !strings.Contains(stdout.String(), "0 issue(s) changed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
