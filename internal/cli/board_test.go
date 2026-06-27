package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withBoardTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := boardStdout
	oldStderr := boardStderr
	oldPlanStderr := planStderr
	boardStdout = stdout
	boardStderr = stderr
	planStderr = stderr
	t.Cleanup(func() {
		boardStdout = oldStdout
		boardStderr = oldStderr
		planStderr = oldPlanStderr
	})
	return stdout, stderr
}

func writeBoardMirrorYAML(t *testing.T, mirrorDir string) {
	t.Helper()
	mirrorYAML := `project:
  type: project
  value: TEST
scopes:
  current-sprint:
    query: "project = TEST AND statusCategory != Done ORDER BY rank"`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror.yml: %v", err)
	}
}

func writeBoardMirrorYAMLSimple(t *testing.T, mirrorDir string) {
	t.Helper()
	mirrorYAML := `project:
  type: project
  value: TEST
scopes:
  current-sprint:
    query: "project = TEST"`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror.yml: %v", err)
	}
}

func writeBoardIssue(t *testing.T, localDir string, key, summary, status string) {
	t.Helper()
	issue := `---
key: ` + key + `
type: story
project:
  type: project
  value: TEST
schema_version: "1"
state: ` + status + `
remote_version: "1"
summary: ` + summary + `
remote:
  jira_base_url: https://jira.example.com
  issue_id: "` + key + `"
  issue_key: ` + key + `
workflow:
  issue_type: Story
relations:
  assignee: null
  reporter: user:alice
  parent: null
  epic: null
  sprint: null
  fix_versions: []
  affects_versions: []
  labels: []
  components: []
links:
  blocks: []
  blocked_by: []
  relates_to: []
  duplicates: []
permissions:
  editable:
    - summary
    - description
---

Body.
`
	filename := key + ".md"
	if err := os.WriteFile(filepath.Join(localDir, filename), []byte(issue), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", filename, err)
	}
}

func writeBoardIssueWithAssignee(t *testing.T, localDir string, key, summary, status, assignee string) {
	t.Helper()
	assigneeLine := "assignee: null"
	if assignee != "" {
		assigneeLine = "assignee: " + assignee
	}
	issue := `---
key: ` + key + `
type: story
project:
  type: project
  value: TEST
schema_version: "1"
state: ` + status + `
remote_version: "1"
summary: ` + summary + `
remote:
  jira_base_url: https://jira.example.com
  issue_id: "` + key + `"
  issue_key: ` + key + `
workflow:
  issue_type: Story
relations:
  ` + assigneeLine + `
  reporter: user:alice
  parent: null
  epic: null
  sprint: null
  fix_versions: []
  affects_versions: []
  labels: []
  components: []
links:
  blocks: []
  blocked_by: []
  relates_to: []
  duplicates: []
permissions:
  editable:
    - summary
    - description
---

Body.
`
	filename := key + ".md"
	if err := os.WriteFile(filepath.Join(localDir, filename), []byte(issue), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", filename, err)
	}
}

func writeBoardIssueWithEpic(t *testing.T, localDir string, key, summary, status, epic string) {
	t.Helper()
	epicLine := "epic: null"
	if epic != "" {
		epicLine = "epic: " + epic
	}
	issue := `---
key: ` + key + `
type: story
project:
  type: project
  value: TEST
schema_version: "1"
state: ` + status + `
remote_version: "1"
summary: ` + summary + `
remote:
  jira_base_url: https://jira.example.com
  issue_id: "` + key + `"
  issue_key: ` + key + `
workflow:
  issue_type: Story
relations:
  assignee: null
  reporter: user:alice
  parent: null
  ` + epicLine + `
  sprint: null
  fix_versions: []
  affects_versions: []
  labels: []
  components: []
links:
  blocks: []
  blocked_by: []
  relates_to: []
  duplicates: []
permissions:
  editable:
    - summary
    - description
---

Body.
`
	filename := key + ".md"
	if err := os.WriteFile(filepath.Join(localDir, filename), []byte(issue), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", filename, err)
	}
}

// TestRunBoard_Help verifies that --help returns exit code 0 and prints
// usage information.
func TestRunBoard_Help(t *testing.T) {
	_, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--help"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--help\"]) = %d, want 0", exit)
	}
	output := stderr.String()
	if !strings.Contains(output, "Usage:") {
		t.Fatalf("stderr should contain 'Usage:', got: %s", output)
	}
	if !strings.Contains(output, "board") {
		t.Fatalf("stderr should contain 'board', got: %s", output)
	}
	if !strings.Contains(output, "--group-by") {
		t.Fatalf("stderr should contain '--group-by', got: %s", output)
	}
	if !strings.Contains(output, "--project") {
		t.Fatalf("stderr should contain '--project', got: %s", output)
	}
	if !strings.Contains(output, "--cwd") {
		t.Fatalf("stderr should contain '--cwd', got: %s", output)
	}
}

// TestRunBoard_HelpShortFlag verifies that -h returns exit code 0.
func TestRunBoard_HelpShortFlag(t *testing.T) {
	withBoardTestIO(t)
	exit := RunBoard([]string{"-h"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"-h\"]) = %d, want 0", exit)
	}
}

// TestRunBoard_InvalidGroupBy verifies that an invalid --group-by value
// returns exit code 1 with a descriptive error.
func TestRunBoard_InvalidGroupBy(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--group-by", "invalid"})
	if exit != 1 {
		t.Fatalf("RunBoard([\"--group-by\", \"invalid\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "invalid --group-by") {
		t.Fatalf("stderr = %q, want 'invalid --group-by'", stderr.String())
	}
}

// TestRunBoard_EmptyState verifies that when no issues exist in the
// local directories, a readable empty-state message is printed.
func TestRunBoard_EmptyState(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Ensure the local directory exists but is empty.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	stdout, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--project\", \"test\"]) = %d, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "no issues found") {
		t.Fatalf("stdout should contain 'no issues found', got: %s", output)
	}
	if !strings.Contains(output, localDir) {
		t.Fatalf("stdout should mention the local directory path, got: %s", output)
	}
}

// TestRunBoard_NoIssuesNoLocalDirs verifies that when a project has no
// local directories configured, a specific message is printed.
func TestRunBoard_NoIssuesNoLocalDirs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write settings with a project that has no local_dirs.
	credsPath := filepath.Join(tmpDir, "creds.toml")
	if err := os.WriteFile(credsPath, []byte("email = \"user@example.com\"\napi_token = \"token\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile creds: %v", err)
	}
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "atlassian_api_token"
credential_refs = ["file://` + credsPath + `"]

[projects.empty]
key = "EMPTY"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	stdout, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "empty"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--project\", \"empty\"]) = %d, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "no local directories configured") {
		t.Fatalf("stdout should contain 'no local directories configured', got: %s", output)
	}
}

// TestRunBoard_WithIssues tests that issues are rendered with stable
// ordering when present in the local directories.
func TestRunBoard_WithIssues(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	writeBoardMirrorYAML(t, mirrorDir)

	// Write issue files using the jirafs schema format.
	writeBoardIssue(t, localDir, "PROJ-1", "First issue", "In Progress")
	writeBoardIssue(t, localDir, "PROJ-10", "Tenth issue", "To Do")
	writeBoardIssue(t, localDir, "PROJ-2", "Second issue", "Done")

	stdout, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--project\", \"test\"]) = %d, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()

	// Verify group line is present.
	if !strings.Contains(output, "jirafs board:") {
		t.Fatalf("stdout should contain 'jirafs board:', got: %s", output)
	}
	if !strings.Contains(output, "group=status") {
		t.Fatalf("stdout should contain 'group=status', got: %s", output)
	}

	// Verify all issue keys appear.
	for _, key := range []string{"PROJ-1", "PROJ-10", "PROJ-2"} {
		if !strings.Contains(output, key) {
			t.Fatalf("stdout should contain %s, got: %s", key, output)
		}
	}

	// Verify issues are sorted by key within columns.
	// PROJ-1 should appear before PROJ-10 which should appear before PROJ-2
	// (lexicographic ordering).
	lastPos := 0
	for _, key := range []string{"PROJ-1", "PROJ-10", "PROJ-2"} {
		pos := strings.Index(output[lastPos:], key)
		if pos < 0 {
			t.Fatalf("could not find %s in output after position %d", key, lastPos)
		}
		lastPos += pos + len(key)
	}
}

// TestRunBoard_GroupByAssignee tests the --group-by=assignee mode.
func TestRunBoard_GroupByAssignee(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	writeBoardMirrorYAMLSimple(t, mirrorDir)

	writeBoardIssueWithAssignee(t, localDir, "PROJ-1", "Assigned to Jesper", "To Do", "user:jesper")
	writeBoardIssueWithAssignee(t, localDir, "PROJ-2", "Assigned to Bob", "In Progress", "user:bob")
	writeBoardIssueWithAssignee(t, localDir, "PROJ-3", "Unassigned", "To Do", "")

	stdout, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "test", "--group-by", "assignee"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--group-by\", \"assignee\"]) = %d, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()

	if !strings.Contains(output, "group=assignee") {
		t.Fatalf("stdout should contain 'group=assignee', got: %s", output)
	}
}

// TestRunBoard_GroupByEpic tests the --group-by=epic mode.
func TestRunBoard_GroupByEpic(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	writeBoardMirrorYAMLSimple(t, mirrorDir)

	writeBoardIssueWithEpic(t, localDir, "PROJ-1", "Epic child 1", "To Do", "PROJ-100")
	writeBoardIssueWithEpic(t, localDir, "PROJ-2", "Epic child 2", "In Progress", "PROJ-100")
	writeBoardIssueWithEpic(t, localDir, "PROJ-3", "No epic", "To Do", "")

	stdout, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "test", "--group-by", "epic"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--group-by\", \"epic\"]) = %d, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()

	if !strings.Contains(output, "group=epic") {
		t.Fatalf("stdout should contain 'group=epic', got: %s", output)
	}
}

// TestRunBoard_NoProjectResolved verifies that when no project can be
// resolved (wrong --cwd), exit code 1 is returned.
func TestRunBoard_NoProjectResolved(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--cwd", filepath.Join(tmpDir, "nowhere")})
	if exit != 1 {
		t.Fatalf("RunBoard([\"--cwd\", \"nowhere\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want 'no project resolved'", stderr.String())
	}
}

// TestRunBoard_MissingProjectInSettings verifies that when the resolved
// project is not in settings, exit code 1 is returned.
func TestRunBoard_MissingProjectInSettings(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Use --project that doesn't exist in settings.
	_, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "nonexistent"})
	if exit != 1 {
		t.Fatalf("RunBoard([\"--project\", \"nonexistent\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in settings") {
		t.Fatalf("stderr = %q, want 'not found in settings'", stderr.String())
	}
}

// TestRunBoard_DefaultGroupByStatus verifies that the default group-by
// is "status" when no --group-by flag is passed.
func TestRunBoard_DefaultGroupByStatus(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	writeBoardMirrorYAMLSimple(t, mirrorDir)

	writeBoardIssue(t, localDir, "PROJ-1", "Default group test", "To Do")

	stdout, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--project\", \"test\"]) = %d, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()

	if !strings.Contains(output, "group=status") {
		t.Fatalf("default group-by should be 'status', got: %s", output)
	}
}

// TestRunBoard_NoIssuesEmptyLocalDirs verifies that when local_dirs
// is empty (no directories at all), the "no local directories" message
// is shown.
func TestRunBoard_NoIssuesEmptyLocalDirs(t *testing.T) {
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

[projects.nodirs]
key = "NODIRS"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
`
		if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	stdout, _ := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "nodirs"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--project\", \"nodirs\"]) = %d, want 0", exit)
	}
	if !strings.Contains(stdout.String(), "no local directories configured") {
		t.Fatalf("stdout should contain 'no local directories configured', got: %s", stdout.String())
	}
}

// TestRunBoard_SkipsNonMarkdownFiles verifies that only .md files are
// processed and non-markdown files are ignored.
func TestRunBoard_SkipsNonMarkdownFiles(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	writeBoardIssue(t, localDir, "PROJ-1", "Valid issue", "To Do")

	// Write non-.md files that should be ignored.
	if err := os.WriteFile(filepath.Join(localDir, "notes.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatalf("WriteFile notes.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "data.json"), []byte(`{"key":"PROJ-99"}`), 0o644); err != nil {
		t.Fatalf("WriteFile data.json: %v", err)
	}

	stdout, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--project\", \"test\"]) = %d, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()

	// Should only see PROJ-1, not PROJ-99.
	if !strings.Contains(output, "PROJ-1") {
		t.Fatalf("stdout should contain PROJ-1, got: %s", output)
	}
	if strings.Contains(output, "PROJ-99") {
		t.Fatalf("stdout should NOT contain PROJ-99 (from non-.md file), got: %s", output)
	}
}

// TestRunBoard_SkipsUnparseableIssues verifies that unparseable .md
// files are silently skipped.
func TestRunBoard_SkipsUnparseableIssues(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	writeBoardIssue(t, localDir, "PROJ-1", "Valid issue", "To Do")

	// Write an unparseable .md file (no --- separator).
	badIssue := `This is not a valid issue file.
No YAML front matter here.`
	if err := os.WriteFile(filepath.Join(localDir, "BAD-1.md"), []byte(badIssue), 0o644); err != nil {
		t.Fatalf("WriteFile BAD-1.md: %v", err)
	}

	stdout, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--project\", \"test\"]) = %d, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()

	// Should only see PROJ-1, not BAD-1 (which is unparseable).
	if !strings.Contains(output, "PROJ-1") {
		t.Fatalf("stdout should contain PROJ-1, got: %s", output)
	}
	if strings.Contains(output, "BAD-1") {
		t.Fatalf("stdout should NOT contain BAD-1 (unparseable), got: %s", output)
	}
}

// TestRunBoard_DuplicateKeysSkipped verifies that duplicate issue keys
// across directories are deduplicated (first seen wins).
func TestRunBoard_DuplicateKeysSkipped(t *testing.T) {
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
	localDir1 := filepath.Join(tmpDir, "local1")
	localDir2 := filepath.Join(tmpDir, "local2")
	if err := os.MkdirAll(localDir1, 0o755); err != nil {
		t.Fatalf("MkdirAll local1: %v", err)
	}
	if err := os.MkdirAll(localDir2, 0o755); err != nil {
		t.Fatalf("MkdirAll local2: %v", err)
	}
	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	writeBoardMirrorYAMLSimple(t, mirrorDir)

	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "atlassian_api_token"
credential_refs = ["file://` + credsPath + `"]

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + mirrorDir + `"
local_dirs = ["` + localDir1 + `", "` + localDir2 + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Same key in both directories.
	issue1 := `---
key: PROJ-1
type: story
project:
  type: project
  value: TEST
schema_version: "1"
state: To Do
remote_version: "1"
summary: From local1
remote:
  jira_base_url: https://jira.example.com
  issue_id: "1"
  issue_key: PROJ-1
workflow:
  issue_type: Story
relations:
  assignee: null
  reporter: user:alice
  parent: null
  epic: null
  sprint: null
  fix_versions: []
  affects_versions: []
  labels: []
  components: []
links:
  blocks: []
  blocked_by: []
  relates_to: []
  duplicates: []
permissions:
  editable:
    - summary
    - description
---

Body from local1.
`
	issue2 := `---
key: PROJ-1
type: story
project:
  type: project
  value: TEST
schema_version: "1"
state: In Progress
remote_version: "1"
summary: From local2
remote:
  jira_base_url: https://jira.example.com
  issue_id: "1"
  issue_key: PROJ-1
workflow:
  issue_type: Story
relations:
  assignee: null
  reporter: user:alice
  parent: null
  epic: null
  sprint: null
  fix_versions: []
  affects_versions: []
  labels: []
  components: []
links:
  blocks: []
  blocked_by: []
  relates_to: []
  duplicates: []
permissions:
  editable:
    - summary
    - description
---

Body from local2.
`
	if err := os.WriteFile(filepath.Join(localDir1, "PROJ-1.md"), []byte(issue1), 0o644); err != nil {
		t.Fatalf("WriteFile local1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir2, "PROJ-1.md"), []byte(issue2), 0o644); err != nil {
		t.Fatalf("WriteFile local2: %v", err)
	}

	stdout, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunBoard([\"--project\", \"test\"]) = %d, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()

	// Should only see PROJ-1 once (from local1, first directory).
	count := strings.Count(output, "PROJ-1")
	if count != 1 {
		t.Fatalf("PROJ-1 should appear exactly once, got %d occurrences in: %s", count, output)
	}
	if !strings.Contains(output, "From local1") {
		t.Fatalf("Should see 'From local1' (first directory), got: %s", output)
	}
}

// TestRunBoard_MissingMirrorDirStillWorks verifies that when the mirror
// directory doesn't exist, the board still works (just without registry
// data for status/user resolution).
func TestRunBoard_MissingMirrorDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Don't create the mirror directory at all.

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	writeBoardIssue(t, localDir, "PROJ-1", "Missing mirror test", "To Do")

	stdout, stderr := withBoardTestIO(t)
	exit := RunBoard([]string{"--project", "test"})
	// Should succeed even without a mirror directory.
	if exit != 0 {
		t.Fatalf("RunBoard([\"--project\", \"test\"]) = %d, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "PROJ-1") {
		t.Fatalf("stdout should contain PROJ-1, got: %s", output)
	}
}
