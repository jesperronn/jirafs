package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdir moves the process working directory into dir for the duration of the
// test and restores the previous directory afterwards. The archive command
// resolves its project from os.Getwd(), so tests must run from within a local
// directory for the configured project to be found.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("Chdir back to %s: %v", old, err)
		}
	})
}

// withArchiveTestIO redirects archiveStdout and archiveStderr to buffers for
// the duration of a test, returning them for assertions.
func withArchiveTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := archiveStdout
	oldStderr := archiveStderr
	archiveStdout = stdout
	archiveStderr = stderr
	t.Cleanup(func() {
		archiveStdout = oldStdout
		archiveStderr = oldStderr
	})
	return stdout, stderr
}

// writeArchiveIssue writes an archive-eligible issue snapshot (resolved and
// synced) into localDir as <key>.md, modeled on writeBoardIssue.
func writeArchiveIssue(t *testing.T, localDir string, key, summary string) {
	t.Helper()
	issue := `---
key: ` + key + `
type: story
project:
  type: project
  value: TEST
schema_version: "1"
state: archived
resolved_status: resolved
remote_version: "1"
content_hash: "abc123"
sync_time: "2026-01-01T00:00:00Z"
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
---

Body.
`
	if err := os.WriteFile(filepath.Join(localDir, key+".md"), []byte(issue), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", key+".md", err)
	}
}

// TestRunArchive_Help verifies that --help returns exit code 0 and prints
// usage information.
func TestRunArchive_Help(t *testing.T) {
	_, stderr := withArchiveTestIO(t)

	code := RunArchive([]string{"--help"})
	if code != 0 {
		t.Fatalf("RunArchive --help = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("stderr = %q, want it to contain usage", stderr.String())
	}
}

// TestRunArchive_Issue verifies that an eligible issue is moved from its live
// path into the project's _archive directory and that a success message is
// printed.
func TestRunArchive_Issue(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	writeArchiveIssue(t, localDir, "ABC-1", "Archived test issue")
	chdir(t, localDir)

	stdout, stderr := withArchiveTestIO(t)

	code := RunArchive([]string{"issue", "ABC-1", "--project", "test"})
	if code != 0 {
		t.Fatalf("RunArchive issue = %d, stderr: %s", code, stderr.String())
	}

	archiveDir := filepath.Join(localDir, "_archive")
	archivedPath := filepath.Join(archiveDir, "ABC-1.md")
	if _, err := os.Stat(archivedPath); err != nil {
		t.Fatalf("expected archived file at %s: %v", archivedPath, err)
	}
	if _, err := os.Stat(filepath.Join(localDir, "ABC-1.md")); !os.IsNotExist(err) {
		t.Fatalf("expected live file to be removed, stat err: %v", err)
	}
	if !strings.Contains(stdout.String(), "jirafs: archived") {
		t.Errorf("stdout = %q, want it to contain %q", stdout.String(), "jirafs: archived")
	}
}

// TestRunArchive_Issue_Missing verifies that archiving a key that does not
// exist in the local directories returns exit code 1.
func TestRunArchive_Issue_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	chdir(t, localDir)

	_, stderr := withArchiveTestIO(t)

	code := RunArchive([]string{"issue", "MISSING", "--project", "test"})
	if code != 1 {
		t.Fatalf("RunArchive issue MISSING = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "not found in local directories") {
		t.Errorf("stderr = %q, want it to mention the issue was not found", stderr.String())
	}
}

// TestRunArchive_List_Empty verifies that listing an archive with no entries
// prints a friendly empty-state message.
func TestRunArchive_List_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	chdir(t, localDir)

	stdout, _ := withArchiveTestIO(t)

	code := RunArchive([]string{"list", "--project", "test"})
	if code != 0 {
		t.Fatalf("RunArchive list = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "no archived issues") {
		t.Errorf("stdout = %q, want it to contain %q", stdout.String(), "no archived issues")
	}
}

// TestRunArchive_List verifies that listing prints every archived snapshot,
// sorted by key.
func TestRunArchive_List(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	archiveDir := filepath.Join(localDir, "_archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatalf("MkdirAll _archive: %v", err)
	}
	for _, name := range []string{"ZED-9.md", "ABC-1.md"} {
		if err := os.WriteFile(filepath.Join(archiveDir, name), []byte("body\n"), 0o644); err != nil {
			t.Fatalf("WriteFile archive %s: %v", name, err)
		}
	}
	chdir(t, localDir)

	stdout, _ := withArchiveTestIO(t)

	code := RunArchive([]string{"list", "--project", "test"})
	if code != 0 {
		t.Fatalf("RunArchive list = %d, want 0", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "ABC-1.md") || !strings.Contains(out, "ZED-9.md") {
		t.Errorf("stdout = %q, want it to list both archived keys", out)
	}
	if strings.Index(out, "ABC-1.md") > strings.Index(out, "ZED-9.md") {
		t.Errorf("stdout = %q, want sorted order (ABC-1 before ZED-9)", out)
	}
}

// TestRunArchive_Issue_ProjectNotFound verifies that an unknown --project
// value returns exit code 1 with a project-not-found message.
func TestRunArchive_Issue_ProjectNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	chdir(t, localDir)

	_, stderr := withArchiveTestIO(t)

	code := RunArchive([]string{"issue", "ABC-1", "--project", "ghost"})
	if code != 1 {
		t.Fatalf("RunArchive issue --project ghost = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "not found in settings") {
		t.Errorf("stderr = %q, want it to mention project not found", stderr.String())
	}
}

// TestRunArchive_NoProjectResolved verifies that listing with no resolvable
// project and no --project returns exit code 1.
func TestRunArchive_NoProjectResolved(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	writeMirror(t, tmpDir)

	// Run from a neutral directory that matches no project's mirror/local dirs.
	chdir(t, t.TempDir())

	_, stderr := withArchiveTestIO(t)

	code := RunArchive([]string{"list"})
	if code != 1 {
		t.Fatalf("RunArchive list (no project) = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Errorf("stderr = %q, want it to mention no project resolved", stderr.String())
	}
}

// TestRunArchive_UnknownSubcommand verifies an unrecognized subcommand returns
// exit code 1 and references the help text.
func TestRunArchive_UnknownSubcommand(t *testing.T) {
	_, stderr := withArchiveTestIO(t)

	code := RunArchive([]string{"bogus"})
	if code != 1 {
		t.Fatalf("RunArchive bogus = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Errorf("stderr = %q, want it to mention unknown subcommand", stderr.String())
	}
}

// TestRunArchive_SettingsLoadError verifies that a missing settings file is
// reported as a load error.
func TestRunArchive_SettingsLoadError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, stderr := withArchiveTestIO(t)

	code := RunArchive([]string{"list"})
	if code != 1 {
		t.Fatalf("RunArchive list (no settings) = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no settings found") {
		t.Errorf("stderr = %q, want it to mention no settings found", stderr.String())
	}
}

// TestRunArchive_Issue_MissingKey verifies that "archive issue" with no key
// returns exit code 1.
func TestRunArchive_Issue_MissingKey(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	chdir(t, localDir)

	_, stderr := withArchiveTestIO(t)

	code := RunArchive([]string{"issue", "--project", "test"})
	if code != 1 {
		t.Fatalf("RunArchive issue (no key) = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "missing issue key") {
		t.Errorf("stderr = %q, want it to mention missing issue key", stderr.String())
	}
}

// TestRunArchive_Issue_CwdFlag verifies that --cwd selects the project instead
// of relying on the process working directory.
func TestRunArchive_Issue_CwdFlag(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	t.Setenv("HOME", homeDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	writeArchiveIssue(t, localDir, "DEF-2", "Cwd flag issue")

	stdout, stderr := withArchiveTestIO(t)

	code := RunArchive([]string{"issue", "DEF-2", "--project", "test", "--cwd", localDir})
	if code != 0 {
		t.Fatalf("RunArchive issue --cwd = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "jirafs: archived") {
		t.Errorf("stdout = %q, want it to contain archived message", stdout.String())
	}
}
