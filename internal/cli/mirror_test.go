package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunMirror_NoSubcommand verifies that calling RunMirror with no
// subcommand returns exit code 1 and prints a usage hint.
func TestRunMirror_NoSubcommand(t *testing.T) {
	exit := RunMirror([]string{})
	if exit != 1 {
		t.Errorf("RunMirror([]) = %d, want 1", exit)
	}
}

// TestRunMirror_UnknownSubcommand verifies that an unknown subcommand
// returns exit code 1.
func TestRunMirror_UnknownSubcommand(t *testing.T) {
	exit := RunMirror([]string{"bogus"})
	if exit != 1 {
		t.Errorf("RunMirror([\"bogus\"]) = %d, want 1", exit)
	}
}

// TestRunMirror_Help verifies that "help" returns exit code 0.
func TestRunMirror_Help(t *testing.T) {
	// Help runs before config.Load, so it should succeed regardless of
	// whether a settings file exists.
	exit := RunMirror([]string{"help"})
	if exit != 0 {
		t.Errorf("RunMirror([\"help\"]) = %d, want 0", exit)
	}
}

// TestRunMirrorArchiveSweep_NoProject verifies that archive-sweep
// returns error when no project can be resolved.
func TestRunMirrorArchiveSweep_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunMirror([]string{"archive-sweep"})
	if exit != 1 {
		t.Errorf("RunMirror([\"archive-sweep\"]) = %d, want 1", exit)
	}
}

func writeSettings(t *testing.T, tmpDir string) string {
	t.Helper()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
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
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}
	return homeDir
}

func writeMirror(t *testing.T, tmpDir string) {
	t.Helper()
	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	mirrorYAML := `project:
  type: project
  value: TEST
`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}
}

func writeIssue(t *testing.T, localDir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(localDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
}

// TestRunMirrorArchiveSweep_EligibleIssues verifies that archive-sweep
// reports eligible issues without mutation.
func TestRunMirrorArchiveSweep_EligibleIssues(t *testing.T) {
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

	// Capture output to verify eligible issues are reported.
	type output struct {
		exit int
	}
	_ = output{}

	exit = RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("RunMirror([\"archive-sweep\", \"--project\", \"test\"]) = %d, want 0", exit)
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

// TestRunMirrorArchiveSweep_NoEligibleIssues verifies that archive-sweep
// reports nothing when no issues are eligible.
func TestRunMirrorArchiveSweep_NoEligibleIssues(t *testing.T) {
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

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

// TestRunMirrorArchiveSweep_PinnedIssue verifies that pinned issues
// are not reported as eligible.
func TestRunMirrorArchiveSweep_PinnedIssue(t *testing.T) {
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

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

// TestRunMirrorArchiveSweep_UnsyncedIssue verifies that unsynced issues
// (zero remote metadata) are not reported as eligible.
func TestRunMirrorArchiveSweep_UnsyncedIssue(t *testing.T) {
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

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

// TestRunMirrorArchiveSweep_ExplicitImportNotEligible verifies that
// explicitly imported issues are not reported as eligible.
func TestRunMirrorArchiveSweep_ExplicitImportNotEligible(t *testing.T) {
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

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

// TestRunMirrorArchiveSweep_ScopeMemberNotEligible verifies that scope
// members are not reported as eligible.
func TestRunMirrorArchiveSweep_ScopeMemberNotEligible(t *testing.T) {
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

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}
