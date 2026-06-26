package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestServiceFunc_Archive(t *testing.T) {
	called := false
	f := ServiceFunc(func(eligible string, mirrorDir, localDir, issuePath string) error {
		called = true
		if eligible != "PROJ-123" {
			t.Errorf("eligible = %q, want %q", eligible, "PROJ-123")
		}
		if mirrorDir != "/mirror" {
			t.Errorf("mirrorDir = %q, want %q", mirrorDir, "/mirror")
		}
		if localDir != "/local" {
			t.Errorf("localDir = %q, want %q", localDir, "/local")
		}
		if issuePath != "/local/PROJ-123.md" {
			t.Errorf("issuePath = %q, want %q", issuePath, "/local/PROJ-123.md")
		}
		return nil
	})

	if err := f.Archive("PROJ-123", "/mirror", "/local", "/local/PROJ-123.md"); err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if !called {
		t.Error("ServiceFunc was not called")
	}
}

func TestFileService_Archive_movesSnapshotAndMarksArchived(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "live")
	archiveDir := filepath.Join(localDir, "_archive")
	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}

	issuePath := filepath.Join(localDir, "PROJ-123.md")
	original := `---
key: PROJ-123
type: story
project: 'project:PROJ'
schema_version: '1'
remote_version: '42'
content_hash: 'hash123'
sync_time: '2026-06-22T10:00:00Z'
resolved_status: 'resolved'
summary: 'Keep my snapshot'
labels:
- 'release'
assignee: 'alice'
linked_issues:
- key: 'PROJ-9'
  type: 'blocks'
---
## Description
Important historical context.

## Acceptance Criteria
- done

## Definition of Ready

## Notes
Preserve this note.

## Comments To Add

## Remote Comments

`
	if err := os.WriteFile(issuePath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile issue: %v", err)
	}

	// Write an empty mirror file so the archive service can load it.
	mirrorPath := filepath.Join(mirrorDir, "mirror.yml")
	if err := os.WriteFile(mirrorPath, []byte("project:\n  type: project\n  value: PROJ\n"), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	svc := FileService{ArchiveDir: archiveDir}
	if err := svc.Archive("PROJ-123", mirrorDir, localDir, issuePath); err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}

	if _, err := os.Stat(issuePath); !os.IsNotExist(err) {
		t.Fatalf("live issue should be removed, stat err = %v", err)
	}

	archivedPath := filepath.Join(archiveDir, "PROJ-123.md")
	data, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Fatalf("ReadFile archived issue: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "state: 'archived'") {
		t.Fatalf("archived snapshot missing archived state:\n%s", got)
	}

	issue, parseErr := schema.ParseIssue(got)
	if parseErr != nil {
		t.Fatalf("ParseIssue archived snapshot: %v\n%s", parseErr, got)
	}
	if issue.RemoteMetadata.State() != schema.StateArchived {
		t.Fatalf("archived snapshot state = %q, want %q", issue.RemoteMetadata.State(), schema.StateArchived)
	}
	if issue.RemoteMetadata.RemoteVersion != "42" {
		t.Fatalf("remote_version = %q, want %q", issue.RemoteMetadata.RemoteVersion, "42")
	}
	if issue.Summary != "Keep my snapshot" {
		t.Fatalf("summary = %q, want %q", issue.Summary, "Keep my snapshot")
	}
	if issue.Sections[schema.SecDescription] != "Important historical context." {
		t.Fatalf("description section = %q", issue.Sections[schema.SecDescription])
	}
	if issue.Sections[schema.SecNotes] != "Preserve this note." {
		t.Fatalf("notes section = %q", issue.Sections[schema.SecNotes])
	}
}

func TestFileService_Archive_keepsLiveFileWhenSnapshotWriteFails(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "live")
	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}

	issuePath := filepath.Join(localDir, "PROJ-124.md")
	content := `---
key: PROJ-124
type: story
project: 'project:PROJ'
remote_version: '1'
content_hash: 'hash'
sync_time: '2026-06-22T10:00:00Z'
resolved_status: 'resolved'
---
## Description
Body.

## Acceptance Criteria

## Definition of Ready

## Notes

## Comments To Add

## Remote Comments

`
	if err := os.WriteFile(issuePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile issue: %v", err)
	}

	// Write an empty mirror file so the archive service can load it.
	mirrorPath := filepath.Join(mirrorDir, "mirror.yml")
	if err := os.WriteFile(mirrorPath, []byte("project:\n  type: project\n  value: PROJ\n"), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	blocker := filepath.Join(tmpDir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile blocker: %v", err)
	}

	svc := FileService{ArchiveDir: blocker}
	err := svc.Archive("PROJ-124", mirrorDir, localDir, issuePath)
	if err == nil {
		t.Fatal("expected archive error for file archive path")
	}

	if _, statErr := os.Stat(issuePath); statErr != nil {
		t.Fatalf("live issue should still exist after failed archive: %v", statErr)
	}
}

func TestFileService_Archive_ReturnsErrorWhenArchiveDirIsEmpty(t *testing.T) {
	svc := FileService{ArchiveDir: ""}
	err := svc.Archive("PROJ-1", "/mirror", "/local", "/local/PROJ-1.md")
	if err == nil {
		t.Fatal("expected error for empty archive directory")
	}
	if !strings.Contains(err.Error(), "archive directory is empty") {
		t.Fatalf("error = %q, want 'archive directory is empty'", err.Error())
	}
}

func TestFileService_Archive_ReturnsErrorWhenIssueFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, "archive")

	svc := FileService{ArchiveDir: archiveDir}
	err := svc.Archive("PROJ-1", filepath.Join(tmpDir, "mirror"), tmpDir, "/nonexistent/PROJ-1.md")
	if err == nil {
		t.Fatal("expected error for missing issue file")
	}
	if !strings.Contains(err.Error(), "read issue file") {
		t.Fatalf("error = %q, want 'read issue file'", err.Error())
	}
}

func TestFileService_Archive_ReturnsErrorForInvalidIssueFile(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "live")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	issuePath := filepath.Join(localDir, "PROJ-999.md")
	invalid := `---
this is not valid frontmatter
---
## Description
Body.
`
	if err := os.WriteFile(issuePath, []byte(invalid), 0o644); err != nil {
		t.Fatalf("WriteFile issue: %v", err)
	}

	svc := FileService{ArchiveDir: filepath.Join(localDir, "_archive")}
	err := svc.Archive("PROJ-999", filepath.Join(tmpDir, "mirror"), localDir, issuePath)
	if err == nil {
		t.Fatal("expected error for invalid issue file")
	}
	if !strings.Contains(err.Error(), "parse issue file") {
		t.Fatalf("error = %q, want 'parse issue file'", err.Error())
	}
}

func TestFileService_Archive_rejectsPinnedIssue(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "live")
	archiveDir := filepath.Join(localDir, "_archive")
	mirrorDir := filepath.Join(tmpDir, "mirror")
	for _, dir := range []string{localDir, archiveDir, mirrorDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}

	issuePath := filepath.Join(localDir, "PROJ-200.md")
	content := `---
key: PROJ-200
type: story
project: 'project:PROJ'
remote_version: '42'
content_hash: 'hash123'
sync_time: '2026-06-22T10:00:00Z'
pinned: true
---
## Description
This issue is pinned.

## Acceptance Criteria

## Definition of Ready

## Notes

## Comments To Add

## Remote Comments

`
	if err := os.WriteFile(issuePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile issue: %v", err)
	}

	// Write an empty mirror file.
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte("project:\n  type: project\n  value: PROJ\n"), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	svc := FileService{ArchiveDir: archiveDir}
	err := svc.Archive("PROJ-200", mirrorDir, localDir, issuePath)
	if err == nil {
		t.Fatal("expected archive error for pinned issue")
	}
	if !strings.Contains(err.Error(), "not archive-eligible") {
		t.Fatalf("error = %q, want 'not archive-eligible'", err.Error())
	}

	// Live file should still exist.
	if _, statErr := os.Stat(issuePath); statErr != nil {
		t.Fatalf("live issue should still exist: %v", statErr)
	}
}

func TestFileService_Archive_rejectsUnsyncedIssue(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "live")
	archiveDir := filepath.Join(localDir, "_archive")
	mirrorDir := filepath.Join(tmpDir, "mirror")
	for _, dir := range []string{localDir, archiveDir, mirrorDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}

	issuePath := filepath.Join(localDir, "PROJ-201.md")
	content := `---
key: PROJ-201
type: story
project: 'project:PROJ'
summary: 'Unsynced issue'
---
## Description
This issue has no remote metadata.

## Acceptance Criteria

## Definition of Ready

## Notes

## Comments To Add

## Remote Comments

`
	if err := os.WriteFile(issuePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile issue: %v", err)
	}

	// Write an empty mirror file.
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte("project:\n  type: project\n  value: PROJ\n"), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	svc := FileService{ArchiveDir: archiveDir}
	err := svc.Archive("PROJ-201", mirrorDir, localDir, issuePath)
	if err == nil {
		t.Fatal("expected archive error for unsynced issue")
	}
	if !strings.Contains(err.Error(), "not archive-eligible") {
		t.Fatalf("error = %q, want 'not archive-eligible'", err.Error())
	}

	// Live file should still exist.
	if _, statErr := os.Stat(issuePath); statErr != nil {
		t.Fatalf("live issue should still exist: %v", statErr)
	}
}

func TestFileService_Archive_rejectsExplicitlyImportedIssue(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "live")
	archiveDir := filepath.Join(localDir, "_archive")
	mirrorDir := filepath.Join(tmpDir, "mirror")
	for _, dir := range []string{localDir, archiveDir, mirrorDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}

	issuePath := filepath.Join(localDir, "PROJ-202.md")
	content := `---
key: PROJ-202
type: story
project: 'project:PROJ'
remote_version: '42'
content_hash: 'hash123'
sync_time: '2026-06-22T10:00:00Z'
---
## Description
This issue is explicitly imported.

## Acceptance Criteria

## Definition of Ready

## Notes

## Comments To Add

## Remote Comments

`
	if err := os.WriteFile(issuePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile issue: %v", err)
	}

	// Write a mirror file with PROJ-202 explicitly imported.
	mirrorContent := `project:
  type: project
  value: PROJ
issues:
- key: PROJ-202
  reason: manual
`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorContent), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	svc := FileService{ArchiveDir: archiveDir}
	err := svc.Archive("PROJ-202", mirrorDir, localDir, issuePath)
	if err == nil {
		t.Fatal("expected archive error for explicitly imported issue")
	}
	if !strings.Contains(err.Error(), "not archive-eligible") {
		t.Fatalf("error = %q, want 'not archive-eligible'", err.Error())
	}

	// Live file should still exist.
	if _, statErr := os.Stat(issuePath); statErr != nil {
		t.Fatalf("live issue should still exist: %v", statErr)
	}
}

func TestFileService_Archive_rejectsScopeMember(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "live")
	archiveDir := filepath.Join(localDir, "_archive")
	mirrorDir := filepath.Join(tmpDir, "mirror")
	for _, dir := range []string{localDir, archiveDir, mirrorDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}

	issuePath := filepath.Join(localDir, "PROJ-203.md")
	content := `---
key: PROJ-203
type: story
project: 'project:PROJ'
remote_version: '42'
content_hash: 'hash123'
sync_time: '2026-06-22T10:00:00Z'
---
## Description
This issue is a scope member.

## Acceptance Criteria

## Definition of Ready

## Notes

## Comments To Add

## Remote Comments

`
	if err := os.WriteFile(issuePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile issue: %v", err)
	}

	// Write a mirror file with PROJ-203 as a scope member.
	mirrorContent := `project:
  type: project
  value: PROJ
scopes:
- name: 'current-sprint'
  type: 'sprint'
  target: 'Sprint-42'
scope_members:
- key: PROJ-203
  scope: 'current-sprint'
`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorContent), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	svc := FileService{ArchiveDir: archiveDir}
	err := svc.Archive("PROJ-203", mirrorDir, localDir, issuePath)
	if err == nil {
		t.Fatal("expected archive error for scope member")
	}
	if !strings.Contains(err.Error(), "not archive-eligible") {
		t.Fatalf("error = %q, want 'not archive-eligible'", err.Error())
	}

	// Live file should still exist.
	if _, statErr := os.Stat(issuePath); statErr != nil {
		t.Fatalf("live issue should still exist: %v", statErr)
	}
}

func TestFileService_Archive_allowsArchivingWhenNoMirrorFile(t *testing.T) {
	// When there is no mirror file, all issues are eligible (empty mirror).
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "live")
	archiveDir := filepath.Join(localDir, "_archive")
	mirrorDir := filepath.Join(tmpDir, "mirror")
	for _, dir := range []string{localDir, archiveDir, mirrorDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}

	issuePath := filepath.Join(localDir, "PROJ-300.md")
	content := `---
key: PROJ-300
type: story
project: 'project:PROJ'
remote_version: '42'
content_hash: 'hash123'
sync_time: '2026-06-22T10:00:00Z'
resolved_status: 'resolved'
---
## Description
Archivable issue with no mirror file.

## Acceptance Criteria

## Definition of Ready

## Notes

## Comments To Add

## Remote Comments

`
	if err := os.WriteFile(issuePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile issue: %v", err)
	}

	// No mirror file exists - archive should succeed.
	svc := FileService{ArchiveDir: archiveDir}
	err := svc.Archive("PROJ-300", mirrorDir, localDir, issuePath)
	if err != nil {
		t.Fatalf("Archive should succeed when no mirror file: %v", err)
	}

	// Live file should be removed.
	if _, statErr := os.Stat(issuePath); !os.IsNotExist(statErr) {
		t.Fatalf("live issue should be removed")
	}

	// Archived file should exist.
	archivedPath := filepath.Join(archiveDir, "PROJ-300.md")
	if _, statErr := os.Stat(archivedPath); statErr != nil {
		t.Fatalf("archived file should exist: %v", statErr)
	}
}
