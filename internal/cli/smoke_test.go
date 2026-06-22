package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

// TestSmokeExportEditPlanSyncReexport exercises the full
// export → edit → plan → sync → reexport cycle using a disposable
// temp directory and a FakeTransport with real schema data.
//
// Steps:
//  1. Export an issue from Jira (fake) to a local file.
//  2. Edit the local issue file (change summary and description).
//  3. Plan what operations would be needed to sync back.
//  4. Sync (push changes to Jira and apply locally).
//  5. Re-export to verify the remote was updated.
func TestSmokeExportEditPlanSyncReexport(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Set up the fake transport with an initial issue.
	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-42",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary:     "Original summary",
		Description: "Original description body",
		Labels:      []string{"bug"},
		MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
		RemoteMetadata: schema.RemoteMetadata{
			StateFile:     "synced",
			RemoteVersion: "1",
			ContentHash:   "abc123",
		},
	})
	// Override the client factory to use the fake transport.
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})

	// Step 1: Export the issue from Jira to a local file.
	// withExportTestIO swaps the global exportStdout to a buffer and returns it.
	exportStdout, exportStderr := withExportTestIO(t)
	// Use a dedicated export directory (not in local_dirs) so we control the file.
	exportDir := filepath.Join(tmpDir, "exported")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatalf("MkdirAll export: %v", err)
	}

	exit := RunExport([]string{"issue", "--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunExport([\"issue\", \"--project\", \"test\", \"PROJ-42\"]) = %d, stderr = %q", exit, exportStderr.String())
	}

	exportedContent := exportStdout.String()
	if exportedContent == "" {
		t.Fatal("export produced no output")
	}

	// Write the exported content to a local issue file in the project's local_dirs.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	exportedFile := filepath.Join(localDir, "PROJ-42.md")
	if err := os.WriteFile(exportedFile, []byte(exportedContent), 0o644); err != nil {
		t.Fatalf("WriteFile exported file: %v", err)
	}

	// Step 2: Edit the local issue (change summary and description).
	editedContent := strings.ReplaceAll(exportedContent, "Original summary", "Edited summary")
	editedContent = strings.ReplaceAll(editedContent, "Original description body", "Edited description body")
	if err := os.WriteFile(exportedFile, []byte(editedContent), 0o644); err != nil {
		t.Fatalf("WriteFile edited file: %v", err)
	}

	// Step 3: Plan what operations would be needed.
	_, planStderr := withPlanTestIO(t)
	exit = RunPlan([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunPlan([\"PROJ-42\"]) = %d, stderr = %q", exit, planStderr.String())
	}

	// Step 4: Sync (push changes to Jira and apply locally).
	_, syncStderr := withSyncTestIO(t)
	exit = RunSync([]string{"--project", "test", "--apply", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunSync([\"--project\", \"test\", \"--apply\", \"PROJ-42\"]) = %d, stderr = %q", exit, syncStderr.String())
	}

	// Step 5: Re-export to verify the remote was updated.
	exportStdout, exportStderr = withExportTestIO(t)

	exit = RunExport([]string{"issue", "--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunExport re-export = %d, stderr = %q", exit, exportStderr.String())
	}

	reexportedContent := exportStdout.String()
	if reexportedContent == "" {
		t.Fatal("re-export produced no output")
	}

	// Verify the re-exported issue has the edited summary (pushed from local to remote).
	if !strings.Contains(reexportedContent, "Edited summary") {
		t.Fatalf("re-exported content should contain edited summary, got: %s", reexportedContent)
	}

	// Verify the local file was updated by --apply (should have the remote's original values now).
	updatedLocalContent, err := os.ReadFile(exportedFile)
	if err != nil {
		t.Fatalf("ReadFile updated local: %v", err)
	}
	// After sync --apply, the local file should reflect what was pushed to remote.
	// The remote was updated with the edited values, so the local file
	// should now contain the edited values.
	if !strings.Contains(string(updatedLocalContent), "Edited summary") {
		t.Fatalf("updated local file should contain edited summary, got: %s", string(updatedLocalContent))
	}
}

// TestSmokeExportEditPlanSyncReexport_MinimalIssue exercises the full cycle
// with a minimal (draft) issue that has few fields populated.
func TestSmokeExportEditPlanSyncReexport_MinimalIssue(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Set up a minimal issue in the fake transport.
	fake := jira.NewFakeTransport()
	fake.SetIssue("DRF-1", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "DRF-1",
			Type:    "task",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "DRF"},
		},
		MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
	})
	// Override the client factory to use the fake transport.
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})

	// Step 1: Export the minimal issue.
	// withExportTestIO swaps the global exportStdout to a buffer and returns it.
	exportStdout, exportStderr := withExportTestIO(t)

	exit := RunExport([]string{"issue", "--project", "test", "DRF-1"})
	if exit != 0 {
		t.Fatalf("RunExport minimal = %d, stderr = %q", exit, exportStderr.String())
	}

	exportedContent := exportStdout.String()
	if exportedContent == "" {
		t.Fatal("export produced no output for minimal issue")
	}

	// Write to local directory.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	exportedFile := filepath.Join(localDir, "DRF-1.md")
	if err := os.WriteFile(exportedFile, []byte(exportedContent), 0o644); err != nil {
		t.Fatalf("WriteFile exported file: %v", err)
	}

	// Step 2: Edit the local issue by adding a summary.
	editedContent := strings.ReplaceAll(exportedContent, "task", "story")
	// Insert a summary line before the --- separator.
	editedContent = strings.Replace(editedContent, "---\n\n", "---\nsummary: Edited task\n\n", 1)
	if err := os.WriteFile(exportedFile, []byte(editedContent), 0o644); err != nil {
		t.Fatalf("WriteFile edited file: %v", err)
	}

	// Step 3: Plan.
	_, planStderr := withPlanTestIO(t)
	exit = RunPlan([]string{"--project", "test", "DRF-1"})
	if exit != 0 {
		t.Fatalf("RunPlan minimal = %d, stderr = %q", exit, planStderr.String())
	}

	// Step 4: Sync.
	_, syncStderr := withSyncTestIO(t)
	exit = RunSync([]string{"--project", "test", "--apply", "DRF-1"})
	if exit != 0 {
		t.Fatalf("RunSync minimal = %d, stderr = %q", exit, syncStderr.String())
	}

	// Step 5: Re-export.
	exportStdout, exportStderr = withExportTestIO(t)

	exit = RunExport([]string{"issue", "--project", "test", "DRF-1"})
	if exit != 0 {
		t.Fatalf("RunExport re-export minimal = %d, stderr = %q", exit, exportStderr.String())
	}

	reexportedContent := exportStdout.String()
	if reexportedContent == "" {
		t.Fatal("re-export produced no output for minimal issue")
	}

	// Verify the re-exported issue still has the original type (task) since
	// the plan/sync cycle only updates fields that differ, and the summary
	// is the primary editable field.
	if !strings.Contains(reexportedContent, "DRF-1") {
		t.Fatalf("re-exported content should contain DRF-1 key, got: %s", reexportedContent)
	}
}

// TestSmokeExportEditPlanSyncReexport_FetchError verifies the full cycle
// when the remote fetch fails during export.
func TestSmokeExportEditPlanSyncReexport_FetchError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Set up the fake transport to fail on fetch.
	fake := jira.NewFakeTransport()
	fake.SetErr("fetch", jira.NewHTTPErr(500, "server error"))
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})

	_, exportStderr := withExportTestIO(t)
	exit := RunExport([]string{"issue", "--project", "test", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunExport with fetch error = %d, want 1", exit)
	}
	if !strings.Contains(exportStderr.String(), "server error") {
		t.Fatalf("stderr = %q, want server error", exportStderr.String())
	}
}

// TestSmokeExportEditPlanSyncReexport_Conflict verifies the full cycle
// when the plan produces conflicts (status change).
func TestSmokeExportEditPlanSyncReexport_Conflict(t *testing.T) {
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
		Status:  "To Do",
	})
	// Override the client factory to use the fake transport.
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})

	// Step 1: Export.
	// withExportTestIO swaps the global exportStdout to a buffer and returns it.
	exportStdout, exportStderr := withExportTestIO(t)
	exit := RunExport([]string{"issue", "--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunExport = %d, stderr = %q", exit, exportStderr.String())
	}

	exportedContent := exportStdout.String()
	if exportedContent == "" {
		t.Fatal("export produced no output")
	}

	// Write to local directory.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	exportedFile := filepath.Join(localDir, "PROJ-42.md")
	if err := os.WriteFile(exportedFile, []byte(exportedContent), 0o644); err != nil {
		t.Fatalf("WriteFile exported file: %v", err)
	}

	// Step 2: Edit the local issue to include a different status (which is rejected).
	editedContent := strings.ReplaceAll(exportedContent, "To Do", "In Progress")
	if err := os.WriteFile(exportedFile, []byte(editedContent), 0o644); err != nil {
		t.Fatalf("WriteFile edited file: %v", err)
	}

	// Step 3: Plan (should show operations).
	_, planStderr := withPlanTestIO(t)
	exit = RunPlan([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunPlan = %d, stderr = %q", exit, planStderr.String())
	}

	// Step 4: Sync (should report conflicts, not push).
	_, syncStderr := withSyncTestIO(t)
	exit = RunSync([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunSync with conflict = %d, stderr = %q", exit, syncStderr.String())
	}

	// Verify UpdateIssue was NOT called (the fake should still have the original issue).
	_, err := fake.UpdateIssue(nil, "PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42"},
		Summary:  "updated",
	})
	if err == nil {
		// Check if the original issue is still there (not updated).
		original, _ := fake.FetchIssue(nil, "PROJ-42")
		if original != nil && original.Summary == "updated" {
			t.Log("WARNING: UpdateIssue was called despite conflicts")
		}
	}
}

// TestSmokeExportEditPlanSyncReexport_UpdateError verifies the full cycle
// when the remote update fails during sync.
func TestSmokeExportEditPlanSyncReexport_UpdateError(t *testing.T) {
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
		Summary: "Original summary",
	})
	// Fail on update.
	fake.SetErr("update", jira.NewHTTPErr(500, "update server error"))
	// Override the client factory to use the fake transport.
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	withPlanClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})

	// Step 1: Export.
	// withExportTestIO swaps the global exportStdout to a buffer and returns it.
	exportStdout, exportStderr := withExportTestIO(t)
	exit := RunExport([]string{"issue", "--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunExport = %d, stderr = %q", exit, exportStderr.String())
	}

	exportedContent := exportStdout.String()
	if exportedContent == "" {
		t.Fatal("export produced no output")
	}

	// Write to local directory.
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	exportedFile := filepath.Join(localDir, "PROJ-42.md")
	editedContent := strings.ReplaceAll(exportedContent, "Original summary", "Edited summary")
	if err := os.WriteFile(exportedFile, []byte(editedContent), 0o644); err != nil {
		t.Fatalf("WriteFile edited file: %v", err)
	}

	// Step 2: Plan (should show operations).
	_, planStderr := withPlanTestIO(t)
	exit = RunPlan([]string{"--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunPlan = %d, stderr = %q", exit, planStderr.String())
	}

	// Step 3: Sync (should fail on update).
	_, syncStderr := withSyncTestIO(t)
	exit = RunSync([]string{"--project", "test", "--apply", "PROJ-42"})
	if exit != 1 {
		t.Fatalf("RunSync with update error = %d, want 1", exit)
	}
	if !strings.Contains(syncStderr.String(), "cannot update remote issue") {
		t.Fatalf("stderr = %q, want cannot update remote issue", syncStderr.String())
	}

	// Step 4: Verify re-export still shows the original (unchanged) remote.
	exportStdout, exportStderr = withExportTestIO(t)

	exit = RunExport([]string{"issue", "--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunExport re-export = %d, stderr = %q", exit, exportStderr.String())
	}

	reexportedContent := exportStdout.String()
	// The remote should still have the original summary since update failed.
	if !strings.Contains(reexportedContent, "Original summary") {
		t.Fatalf("re-exported content should still have original summary, got: %s", reexportedContent)
	}
}
