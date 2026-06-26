package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
)

// TestUseSnapshot_ZeroValue verifies that a zero-value UseSnapshot
// reports IsZero() as true and has no meaningful fields set.
func TestUseSnapshot_ZeroValue(t *testing.T) {
	var snap UseSnapshot
	if !snap.IsZero() {
		t.Fatal("zero-value UseSnapshot should report IsZero() == true")
	}
	if snap.ProjectName != "" || snap.ProjectKey != "" ||
		snap.MirrorDir != "" || snap.Instance != "" ||
		snap.Resolved {
		t.Fatal("zero-value UseSnapshot should have no meaningful fields set")
	}
}

// TestBuildUseSnapshot_NoSettings verifies that BuildUseSnapshot
// with nil settings returns a snapshot where Resolved is false.
func TestBuildUseSnapshot_NoSettings(t *testing.T) {
	snap := BuildUseSnapshot(nil, "/some/cwd")
	if snap.Resolved {
		t.Fatal("expected Resolved == false with nil settings")
	}
	if !snap.IsZero() {
		t.Fatal("expected IsZero() == true with nil settings")
	}
}

// TestBuildUseSnapshot_NoProjectResolved verifies that when settings
// exist but no project matches the cwd, Resolved is false.
func TestBuildUseSnapshot_NoProjectResolved(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	snap := BuildUseSnapshot(settings, filepath.Join(tmpDir, "outside"))
	if snap.Resolved {
		t.Fatal("expected Resolved == false when no project matches cwd")
	}
}

// TestBuildUseSnapshot_FullyResolved verifies that when settings exist
// and a project is resolved from within the mirror directory,
// Resolved is true with correct fields.
func TestBuildUseSnapshot_FullyResolved(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write settings with credentials.
	credsPath := filepath.Join(tmpDir, "creds.toml")
	if err := os.WriteFile(credsPath, []byte("email = \"user@example.com\"\napi_token = \"token\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile creds: %v", err)
	}
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "atlassian_api_token"
credential_refs = ["file://` + credsPath + `"]

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	snap := BuildUseSnapshot(settings, filepath.Join(tmpDir, "mirror"))
	if !snap.Resolved {
		t.Fatal("expected Resolved == true")
	}
	if snap.ProjectName != "test" {
		t.Fatalf("expected ProjectName == 'test', got %q", snap.ProjectName)
	}
	if snap.ProjectKey != "TEST" {
		t.Fatalf("expected ProjectKey == 'TEST', got %q", snap.ProjectKey)
	}
	if snap.Instance != "default" {
		t.Fatalf("expected Instance == 'default', got %q", snap.Instance)
	}
	if !strings.Contains(snap.MirrorDir, "mirror") {
		t.Fatalf("expected MirrorDir to contain 'mirror', got %q", snap.MirrorDir)
	}
}

// TestBuildUseSnapshot_ExplicitFlag verifies that BuildUseSnapshot
// with an explicit project flag resolves by that flag.
func TestBuildUseSnapshot_ExplicitFlag(t *testing.T) {
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

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	// Use a resolver with explicit flag "test".
	resolver := context.NewResolver(settings, "test")
	ctx, err := resolver.Resolve("/some/random/path")
	if err != nil {
		t.Fatalf("Resolver.Resolve: %v", err)
	}
	if ctx.Name != "test" || ctx.Key != "TEST" {
		t.Fatalf("expected explicit flag resolution, got name=%q key=%q", ctx.Name, ctx.Key)
	}
}

// TestRunUse_NoArgs verifies that calling RunUse with no arguments
// shows the current project status (not help) and returns exit code 0.
func TestRunUse_NoArgs(t *testing.T) {
	// Set up a temp directory with settings, but don't change to the
	// mirror directory so the project doesn't resolve.
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	stdout, stderr := withUseTestIO(t)
	exit := RunUse([]string{})
	if exit != 0 {
		t.Fatalf("RunUse([]) = %d, want 0", exit)
	}
	output := stdout.String()
	// When no project is resolved (outside mirror dir), should show
	// "no current project set".
	if !strings.Contains(output, "no current project") {
		t.Fatalf("expected 'no current project' in output, got stdout=%q stderr=%q", output, stderr.String())
	}
}

// TestRunUse_Clear verifies that --clear clears the remembered project.
func TestRunUse_Clear(t *testing.T) {
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

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// First set a current project.
	s, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	s.State.CurrentProject = "test"
	if err := s.SaveState(); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	stdout, stderr := withUseTestIO(t)
	exit := RunUse([]string{"--clear"})
	if exit != 0 {
		t.Fatalf("RunUse([\"--clear\"]) = %d, want 0, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "current project cleared") {
		t.Fatalf("expected 'current project cleared' in output, got stdout=%q", output)
	}

	// Verify the state was actually cleared.
	s2, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load after clear: %v", err)
	}
	if s2.State.CurrentProject != "" {
		t.Fatalf("expected CurrentProject to be empty after clear, got %q", s2.State.CurrentProject)
	}
}

// TestRunUse_SetProject verifies that RunUse with a project key
// sets the current project correctly.
func TestRunUse_SetProject(t *testing.T) {
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

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	stdout, stderr := withUseTestIO(t)
	exit := RunUse([]string{"test"})
	if exit != 0 {
		t.Fatalf("RunUse([\"test\"]) = %d, want 0, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "current project set to") {
		t.Fatalf("expected 'current project set to' in output, got stdout=%q", output)
	}
	if !strings.Contains(output, "test") {
		t.Fatalf("expected 'test' in output, got stdout=%q", output)
	}

	// Verify the state was actually set.
	s2, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load after set: %v", err)
	}
	if s2.State.CurrentProject != "test" {
		t.Fatalf("expected CurrentProject == 'test', got %q", s2.State.CurrentProject)
	}
}

// TestRunUse_SetProject_WithFlag verifies --project flag works.
func TestRunUse_SetProject_WithFlag(t *testing.T) {
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

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	stdout, stderr := withUseTestIO(t)
	exit := RunUse([]string{"--project", "test"})
	if exit != 0 {
		t.Fatalf("RunUse([\"--project\", \"test\"]) = %d, want 0, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "current project set to") {
		t.Fatalf("expected 'current project set to' in output, got stdout=%q", output)
	}
}

// TestRunUse_UnknownProject verifies that setting an unknown project
// returns exit code 1.
func TestRunUse_UnknownProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withUseTestIO(t)
	exit := RunUse([]string{"nonexistent"})
	if exit != 1 {
		t.Fatalf("RunUse([\"nonexistent\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not found in settings") {
		t.Fatalf("stderr = %q, want 'not found in settings'", stderr.String())
	}
}

// TestRunUse_ClearAndProjectMutual verifies that --clear and --project
// together returns an error.
func TestRunUse_ClearAndProjectMutual(t *testing.T) {
	_, stderr := withUseTestIO(t)
	exit := RunUse([]string{"--clear", "--project", "test"})
	if exit != 1 {
		t.Fatalf("RunUse([\"--clear\", \"--project\", \"test\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "mutually exclusive") {
		t.Fatalf("stderr = %q, want 'mutually exclusive'", stderr.String())
	}
}

// TestRunUse_TooManyArgs verifies that too many positional arguments
// returns exit code 1.
func TestRunUse_TooManyArgs(t *testing.T) {
	_, stderr := withUseTestIO(t)
	exit := RunUse([]string{"test", "other"})
	if exit != 1 {
		t.Fatalf("RunUse([\"test\", \"other\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "too many positional") {
		t.Fatalf("stderr = %q, want 'too many positional'", stderr.String())
	}
}

// TestRunUse_ShowResolvedProject verifies that when a project is
// resolved (from within the mirror directory), RunUse without args
// shows the resolved project info.
func TestRunUse_ShowResolvedProject(t *testing.T) {
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

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Create directories first, then resolve symlinks so the path matches
	// what os.Getwd() returns (on macOS /var → /private/var).
	mirrorDir := filepath.Join(tmpDir, "mirror")
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}
	resolvedMirror, err := filepath.EvalSymlinks(mirrorDir)
	if err != nil {
		t.Fatalf("EvalSymlinks mirror: %v", err)
	}
	resolvedLocal, err := filepath.EvalSymlinks(localDir)
	if err != nil {
		t.Fatalf("EvalSymlinks local: %v", err)
	}

	// Rewrite settings TOML with resolved paths so config.Load() stores
	// paths matching os.Getwd().
	resolvedSettingsTOML := strings.ReplaceAll(settingsTOML,
		filepath.Join(tmpDir, "mirror"), resolvedMirror)
	resolvedSettingsTOML = strings.ReplaceAll(resolvedSettingsTOML,
		filepath.Join(tmpDir, "local"), resolvedLocal)
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(resolvedSettingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(resolvedMirror); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(oldCwd)

	stdout, stderr := withUseTestIO(t)
	exit := RunUse([]string{})
	if exit != 0 {
		t.Fatalf("RunUse([]) from mirror dir = %d, want 0, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()
	// Should show the resolved project, not "no current project set".
	if strings.Contains(output, "no current project") {
		t.Fatalf("expected resolved project info, got 'no current project': stdout=%q", output)
	}
	if !strings.Contains(output, "current project is") {
		t.Fatalf("expected 'current project is' in output, got stdout=%q", output)
	}
	if !strings.Contains(output, "test") {
		t.Fatalf("expected 'test' in output, got stdout=%q", output)
	}
}

// withUseTestIO captures stdout and stderr for the use command.
func withUseTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := useStdout
	oldStderr := useStderr
	useStdout = stdout
	useStderr = stderr
	t.Cleanup(func() {
		useStdout = oldStdout
		useStderr = oldStderr
	})
	return stdout, stderr
}
