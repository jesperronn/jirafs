package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/archive"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
)

func TestRunArchiveSweep_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunArchive([]string{"sweep"})
	if exit != 1 {
		t.Errorf("RunArchive([\"sweep\"]) = %d, want 1", exit)
	}
}

func TestRunArchiveSweep_EligibleIssues(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Issue 1: resolved, out of scope → eligible.
	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: First issue
`)

	// Issue 2: open → not eligible.
	writeIssue(t, localDir, "TEST-2.md", `---
key: TEST-2
type: bug
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "def"
  resolved_status: "open"
---

Summary: Second issue
`)

	// Issue 3: resolved, out of scope → eligible.
	writeIssue(t, localDir, "TEST-3.md", `---
key: TEST-3
type: task
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "ghi"
  resolved_status: "resolved"
---

Summary: Third issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunArchive([]string{"sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("RunArchive([\"sweep\", \"--project\", \"test\"]) = %d, want 0", exit)
	}

	// Verify files were NOT modified (no mutation).
	for _, name := range []string{"TEST-1.md", "TEST-2.md", "TEST-3.md"} {
		data, err := os.ReadFile(filepath.Join(localDir, name))
		if err != nil {
			t.Errorf("ReadFile %s: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s was emptied by archive-sweep", name)
		}
	}
}

func TestRunArchiveSweep_NoEligibleIssues(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "open"
---

Summary: Open issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunArchive([]string{"sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

func TestRunArchiveSweep_PinnedIssue(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
  pinned: true
---

Summary: Pinned issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunArchive([]string{"sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

func TestRunArchiveSweep_UnsyncedIssue(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
---

Summary: Unsynced issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunArchive([]string{"sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

func TestRunArchiveSweep_ExplicitImportNotEligible(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Mirror with an explicitly imported issue.
	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	mirrorYAML := `project:
  type: project
  value: TEST
issues:
  - key: TEST-1
    reason: manual
`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Issue 1 is explicitly imported + resolved → NOT eligible.
	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: Explicitly imported issue
`)

	// Issue 2 is out of scope + resolved → eligible.
	writeIssue(t, localDir, "TEST-2.md", `---
key: TEST-2
type: bug
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "def"
  resolved_status: "resolved"
---

Summary: Out-of-scope issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunArchive([]string{"sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

func TestRunArchiveSweep_ScopeMemberNotEligible(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Mirror with a scope member.
	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	mirrorYAML := `project:
  type: project
  value: TEST
scopes:
  - name: active
    type: jql
    target: status=Active
scope_members:
  - key: TEST-1
    scope: active
`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Issue 1 is a scope member + resolved → NOT eligible.
	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: Scope member issue
`)

	// Issue 2 is out of scope + resolved → eligible.
	writeIssue(t, localDir, "TEST-2.md", `---
key: TEST-2
type: bug
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "def"
  resolved_status: "resolved"
---

Summary: Out-of-scope issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunArchive([]string{"sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

func TestArchiveSweep_UnparseableIssue(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Write a file with invalid frontmatter.
	if err := os.WriteFile(filepath.Join(localDir, "INVALID.md"), []byte(`---
key:
  invalid: yaml
---

Summary: Invalid issue
`), 0o644); err != nil {
		t.Fatalf("WriteFile INVALID.md: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunArchive([]string{"sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

func TestRunArchiveSweep_Apply_Success(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Write two eligible issues.
	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: First issue
`)

	writeIssue(t, localDir, "TEST-2.md", `---
key: TEST-2
type: bug
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "def"
  resolved_status: "resolved"
---

Summary: Second issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	stdout, stderr := withArchiveTestIO(t)

	exit := RunArchive([]string{"sweep", "--apply", "--project", "test"})
	if exit != 0 {
		t.Fatalf("RunArchive([\"sweep\", \"--apply\", \"--project\", \"test\"]) = %d, want 0, stderr = %q", exit, stderr.String())
	}

	// Verify stdout contains archived messages.
	if !strings.Contains(stdout.String(), "archived: TEST-1") {
		t.Fatalf("stdout = %q, want \"archived: TEST-1\"", stdout.String())
	}
	if !strings.Contains(stdout.String(), "archived: TEST-2") {
		t.Fatalf("stdout = %q, want \"archived: TEST-2\"", stdout.String())
	}

	// Verify files were moved to archive, not deleted.
	archiveDir := filepath.Join(localDir, "_archive")
	if _, err := os.Stat(archiveDir); err != nil {
		t.Fatalf("archive directory should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "TEST-1.md")); err != nil {
		t.Fatalf("TEST-1.md should exist in archive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "TEST-2.md")); err != nil {
		t.Fatalf("TEST-2.md should exist in archive: %v", err)
	}
	archivedData, err := os.ReadFile(filepath.Join(archiveDir, "TEST-1.md"))
	if err != nil {
		t.Fatalf("ReadFile archived TEST-1.md: %v", err)
	}
	if !strings.Contains(string(archivedData), "state: 'archived'") {
		t.Fatalf("archived TEST-1.md = %q, want archived state", string(archivedData))
	}

	// Verify original files no longer exist in local.
	if _, err := os.Stat(filepath.Join(localDir, "TEST-1.md")); !os.IsNotExist(err) {
		t.Error("TEST-1.md should be removed from local")
	}
	if _, err := os.Stat(filepath.Join(localDir, "TEST-2.md")); !os.IsNotExist(err) {
		t.Error("TEST-2.md should be removed from local")
	}
}

func TestRunArchiveSweep_Apply_NoEligible(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Write an open issue (not eligible).
	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "open"
---

Summary: Open issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	stdout, stderr := withArchiveTestIO(t)

	exit := RunArchive([]string{"sweep", "--apply", "--project", "test"})
	if exit != 0 {
		t.Fatalf("RunArchive([\"sweep\", \"--apply\", \"--project\", \"test\"]) = %d, want 0, stderr = %q", exit, stderr.String())
	}
	if want := "jirafs archive sweep: no eligible issues found"; !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout = %q, want containing %q", stdout.String(), want)
	}
}

func TestRunArchiveSweep_Apply_ArchiveDirCreated(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Write an eligible issue.
	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: First issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, _ = withArchiveTestIO(t)

	// No _archive directory exists yet.
	if _, err := os.Stat(filepath.Join(localDir, "_archive")); !os.IsNotExist(err) {
		t.Fatal("_archive directory should not exist yet")
	}

	exit := RunArchive([]string{"sweep", "--apply", "--project", "test"})
	if exit != 0 {
		t.Fatal("expected exit 0")
	}

	// Archive directory should now exist.
	if _, err := os.Stat(filepath.Join(localDir, "_archive")); err != nil {
		t.Fatalf("_archive directory should exist after --apply: %v", err)
	}
}

func TestRunArchiveSweep_Apply_MultipleLocalDirs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}

	localDir1 := filepath.Join(tmpDir, "local1")
	localDir2 := filepath.Join(tmpDir, "local2")
	if err := os.MkdirAll(localDir1, 0o755); err != nil {
		t.Fatalf("MkdirAll local1: %v", err)
	}
	if err := os.MkdirAll(localDir2, 0o755); err != nil {
		t.Fatalf("MkdirAll local2: %v", err)
	}

	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + mirrorDir + `"
local_dirs = ["` + localDir1 + `", "` + localDir2 + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	mirrorYAML := `project:
  type: project
  value: TEST
`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	// Write an eligible issue in localDir1.
	writeIssue(t, localDir1, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: First issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, _ = withArchiveTestIO(t)

	exit := RunArchive([]string{"sweep", "--apply", "--project", "test"})
	if exit != 0 {
		t.Fatal("expected exit 0")
	}

	// The file should be archived in localDir1 (first local dir).
	archiveDir := filepath.Join(localDir1, "_archive")
	if _, err := os.Stat(archiveDir); err != nil {
		t.Fatalf("_archive in localDir1 should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "TEST-1.md")); err != nil {
		t.Fatalf("TEST-1.md should be in archive: %v", err)
	}
}

func TestRunArchiveSweep_Apply_FailedArchiveError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Write an eligible issue.
	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: First issue
`)

	// Create the archive directory and make it unwritable.
	archiveDir := filepath.Join(localDir, "_archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatalf("MkdirAll archive: %v", err)
	}
	// Make the archive directory unwritable so archiving fails.
	if err := os.Chmod(archiveDir, 0o000); err != nil {
		t.Fatalf("Chmod archive: %v", err)
	}
	defer os.Chmod(archiveDir, 0o755) // Restore for cleanup

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	stdout, stderr := withArchiveTestIO(t)

	exit := RunArchive([]string{"sweep", "--apply", "--project", "test"})
	// Should return 1 due to archive failure.
	if exit == 0 {
		t.Fatalf("expected non-zero exit on archive failure")
	}

	// Should contain error message.
	if !strings.Contains(stderr.String(), "error:") {
		t.Fatalf("stderr = %q, want error messages", stderr.String())
	}

	// Should report partial success count.
	if !strings.Contains(stderr.String(), "0/1 issues archived") {
		t.Fatalf("stderr = %q, want 0/1 count", stderr.String())
	}

	// Original file should still exist (not partially moved).
	if _, err := os.Stat(filepath.Join(localDir, "TEST-1.md")); err != nil {
		t.Fatal("original file should still exist after failed archive")
	}

	// Check stdout shows eligible but no archived messages.
	if strings.Contains(stdout.String(), "archived:") {
		t.Fatalf("stdout = %q, should not contain archived messages", stdout.String())
	}
}

func withArchiveServiceFactory(t *testing.T, factory func(*config.Settings, *context.Context, string) (archive.Service, error)) {
	t.Helper()
	oldFactory := archiveServiceFactory
	archiveServiceFactory = factory
	t.Cleanup(func() {
		archiveServiceFactory = oldFactory
	})
}

func TestRunArchiveSweep_ProjectFlagNotInSettings(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withArchiveTestIO(t)

	// --project references a project that does not exist in settings.
	exit := RunArchive([]string{"sweep", "--project", "nonexistent"})
	if exit != 1 {
		t.Fatalf("RunArchive([\"sweep\", \"--project\", \"nonexistent\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "project \"nonexistent\" not found") {
		t.Fatalf("stderr = %q, want project not found", stderr.String())
	}
}

func TestRunArchiveSweep_Apply_ServiceCreationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: First issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	withArchiveServiceFactory(t, func(*config.Settings, *context.Context, string) (archive.Service, error) {
		return nil, os.ErrPermission
	})

	_, stderr := withArchiveTestIO(t)

	exit := RunArchive([]string{"sweep", "--apply", "--project", "test"})
	if exit != 1 {
		t.Fatalf("RunArchive([\"sweep\", \"--apply\", \"--project\", \"test\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create archive service") {
		t.Fatalf("stderr = %q, want archive service creation error", stderr.String())
	}
}

func TestRunArchiveSweep_CwdNotMatching(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withArchiveTestIO(t)

	// --cwd points to a directory that does not match the project's mirror_dir.
	exit := RunArchive([]string{"sweep", "--cwd", filepath.Join(tmpDir, "outside")})
	if exit != 1 {
		t.Fatalf("RunArchive([\"sweep\", \"--cwd\", outside]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}
