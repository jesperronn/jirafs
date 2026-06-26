package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/mirror"
)

// TestStatusSnapshot_ZeroValue verifies that a zero-value StatusSnapshot
// reports IsZero() as true and has no meaningful fields set.
func TestStatusSnapshot_ZeroValue(t *testing.T) {
	var snap StatusSnapshot
	if !snap.IsZero() {
		t.Fatal("zero-value StatusSnapshot should report IsZero() == true")
	}
	if snap.ProjectName != "" || snap.ProjectKey != "" ||
		snap.MirrorDir != "" || snap.Instance != "" ||
		snap.Resolved || snap.MirrorExists {
		t.Fatal("zero-value StatusSnapshot should have no meaningful fields set")
	}
	if len(snap.MissingSteps) != 0 {
		t.Fatal("zero-value StatusSnapshot should have no missing steps")
	}
	if snap.NextStep() != "" {
		t.Fatal("zero-value StatusSnapshot should have empty NextStep()")
	}
}

// TestBuildStatusSnapshot_NoSettings verifies that BuildStatusSnapshot
// with a nil settings returns a snapshot where Resolved is false and
// the only missing step is "settings.toml not found".
func TestBuildStatusSnapshot_NoSettings(t *testing.T) {
	snap := BuildStatusSnapshot(nil, "/some/cwd")
	if snap.Resolved {
		t.Fatal("expected Resolved == false with nil settings")
	}
	if len(snap.MissingSteps) != 1 {
		t.Fatalf("expected 1 missing step, got %d: %v", len(snap.MissingSteps), snap.MissingSteps)
	}
	if snap.OnboardingComplete {
		t.Fatal("expected OnboardingComplete == false")
	}
}

// TestBuildStatusSnapshot_NoProjectResolved verifies that when settings
// exist but no project matches the cwd, Resolved is false and a
// "project not resolved" step is listed.
func TestBuildStatusSnapshot_NoProjectResolved(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	snap := BuildStatusSnapshot(settings, filepath.Join(tmpDir, "outside"))
	if snap.Resolved {
		t.Fatal("expected Resolved == false when no project matches cwd")
	}
	if !snap.HasStep("project not resolved") {
		t.Fatalf("expected 'project not resolved' in missing steps, got: %v", snap.MissingSteps)
	}
}

// TestBuildStatusSnapshot_FullyResolved verifies that when settings exist,
// a project is resolved, and a mirror.yml exists with scopes and issues,
// all onboarding steps are complete.
func TestBuildStatusSnapshot_FullyResolved(t *testing.T) {
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

	// Write a mirror.yml with scopes and issues.
	mirrorDir := filepath.Join(tmpDir, "mirror")
	mirrorYAML := `project:
  type: project
  value: TEST
scopes:
  - name: sprint-1
    type: project
    target: TEST
issues:
  - key: TEST-1
    reason: manual
  - key: TEST-2
    reason: dependency
scope_members:
  - key: TEST-3
    scope: sprint-1
`
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror.yml: %v", err)
	}

	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	snap := BuildStatusSnapshot(settings, filepath.Join(tmpDir, "mirror"))
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
	if !snap.MirrorExists {
		t.Fatal("expected MirrorExists == true")
	}
	if len(snap.MirrorScopes) != 1 {
		t.Fatalf("expected 1 scope, got %d: %v", len(snap.MirrorScopes), snap.MirrorScopes)
	}
	if snap.MirrorScopes[0] != "sprint-1" {
		t.Fatalf("expected scope name 'sprint-1', got %q", snap.MirrorScopes[0])
	}
	if snap.MirrorIssueCount != 2 {
		t.Fatalf("expected 2 issues, got %d", snap.MirrorIssueCount)
	}
	if snap.MirrorScopeMemberCount != 1 {
		t.Fatalf("expected 1 scope member, got %d", snap.MirrorScopeMemberCount)
	}
	if !snap.OnboardingComplete {
		t.Fatalf("expected OnboardingComplete == true, missing steps: %v", snap.MissingSteps)
	}
	if snap.NextStep() != "" {
		t.Fatalf("expected empty NextStep(), got %q", snap.NextStep())
	}
}

// TestBuildStatusSnapshot_MissingCredentials verifies that when a project
// is resolved but its instance has no credential refs, a missing step
// for credentials is reported.
func TestBuildStatusSnapshot_MissingCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write settings without credential refs.
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

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	snap := BuildStatusSnapshot(settings, filepath.Join(tmpDir, "mirror"))
	if !snap.Resolved {
		t.Fatal("expected Resolved == true")
	}
	if !snap.HasStep("no credentials") {
		t.Fatalf("expected 'no credentials' in missing steps, got: %v", snap.MissingSteps)
	}
}

// TestBuildStatusSnapshot_NoMirrorFile verifies that when the project is
// resolved but mirror.yml doesn't exist, MirrorExists is false and a
// missing step is reported.
func TestBuildStatusSnapshot_NoMirrorFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Don't write a mirror.yml — just the settings.
	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	snap := BuildStatusSnapshot(settings, filepath.Join(tmpDir, "mirror"))
	if !snap.Resolved {
		t.Fatal("expected Resolved == true")
	}
	if snap.MirrorExists {
		t.Fatal("expected MirrorExists == false when mirror.yml doesn't exist")
	}
	if !snap.HasStep("mirror.yml not found") {
		t.Fatalf("expected 'mirror.yml not found' in missing steps, got: %v", snap.MissingSteps)
	}
}

// TestBuildStatusSnapshot_NoScopes verifies that when mirror.yml exists
// but defines no scopes, a missing step is reported.
func TestBuildStatusSnapshot_NoScopes(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Write a mirror.yml with no scopes or issues.
	mirrorDir := filepath.Join(tmpDir, "mirror")
	mirrorYAML := `project:
  type: project
  value: TEST
`
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror.yml: %v", err)
	}

	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	snap := BuildStatusSnapshot(settings, filepath.Join(tmpDir, "mirror"))
	if !snap.Resolved {
		t.Fatal("expected Resolved == true")
	}
	if !snap.HasStep("no scopes defined") {
		t.Fatalf("expected 'no scopes defined' in missing steps, got: %v", snap.MissingSteps)
	}
	if !snap.HasStep("no issues imported") {
		t.Fatalf("expected 'no issues imported' in missing steps, got: %v", snap.MissingSteps)
	}
}

// TestStatusSnapshot_HasStep verifies the HasStep method.
func TestStatusSnapshot_HasStep(t *testing.T) {
	snap := StatusSnapshot{
		MissingSteps: []string{
			"settings.toml not found",
			"no Jira instance configured",
		},
	}
	if !snap.HasStep("settings.toml") {
		t.Fatal("expected HasStep('settings.toml') == true")
	}
	if !snap.HasStep("instance configured") {
		t.Fatal("expected HasStep('instance configured') == true")
	}
	if snap.HasStep("nonexistent") {
		t.Fatal("expected HasStep('nonexistent') == false")
	}
}

// TestStatusSnapshot_NextStep verifies the NextStep method.
func TestStatusSnapshot_NextStep(t *testing.T) {
	snap := StatusSnapshot{
		MissingSteps: []string{"step one", "step two", "step three"},
	}
	if snap.NextStep() != "step one" {
		t.Fatalf("expected NextStep() == 'step one', got %q", snap.NextStep())
	}

	complete := StatusSnapshot{
		MissingSteps: []string{},
	}
	if complete.NextStep() != "" {
		t.Fatalf("expected empty NextStep() for complete snapshot, got %q", complete.NextStep())
	}
}

// TestBuildStatusSnapshot_NoInstances verifies that when settings exist
// but no instances are configured, a missing step is reported.
func TestBuildStatusSnapshot_NoInstances(t *testing.T) {
	// Build a Settings struct directly (config.Load() rejects no-instances).
	settings := &config.Settings{
		Version:  1,
		Projects: map[string]config.Project{
			"test": {Key: "TEST", Instance: "default", MirrorDir: "/tmp/mirror", LocalDirs: []string{"/tmp/local"}},
		},
	}

	snap := BuildStatusSnapshot(settings, "/tmp/mirror")
	if !snap.HasStep("no Jira instance configured") {
		t.Fatalf("expected 'no Jira instance configured' in missing steps, got: %v", snap.MissingSteps)
	}
}

// TestBuildStatusSnapshot_NoProjects verifies that when settings exist
// but no projects are configured, a missing step is reported.
func TestBuildStatusSnapshot_NoProjects(t *testing.T) {
	// Build a Settings struct directly (config.Load() rejects no-projects).
	settings := &config.Settings{
		Version:   1,
		Instances: map[string]config.Instance{
			"default": {
				BaseURL:      "https://example.atlassian.net",
				AuthType:     "basic",
				CredentialRefs: []string{"file:///tmp/creds.toml"},
			},
		},
		Projects: map[string]config.Project{},
	}

	snap := BuildStatusSnapshot(settings, "/some/cwd")
	if !snap.HasStep("no project configured") {
		t.Fatalf("expected 'no project configured' in missing steps, got: %v", snap.MissingSteps)
	}
}

// TestReadMirrorFile_Valid verifies that readMirrorFile correctly parses
// a valid mirror.yml file.
func TestReadMirrorFile_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	mirrorYAML := `project:
  type: project
  value: TEST
scopes:
  - name: sprint-1
    type: project
    target: TEST
issues:
  - key: TEST-1
    reason: manual
`
	path := filepath.Join(tmpDir, "mirror.yml")
	if err := os.WriteFile(path, []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m, err := readMirrorFile(path)
	if err != nil {
		t.Fatalf("readMirrorFile: %v", err)
	}
	if m.Project.Value != "TEST" {
		t.Fatalf("expected project value 'TEST', got %q", m.Project.Value)
	}
	if len(m.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(m.Scopes))
	}
	if m.Scopes[0].Name != "sprint-1" {
		t.Fatalf("expected scope name 'sprint-1', got %q", m.Scopes[0].Name)
	}
	if len(m.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(m.Issues))
	}
	if m.Issues[0].Key != "TEST-1" {
		t.Fatalf("expected issue key 'TEST-1', got %q", m.Issues[0].Key)
	}
}

// TestReadMirrorFile_MissingFile verifies that readMirrorFile returns
// an error when the file does not exist.
func TestReadMirrorFile_MissingFile(t *testing.T) {
	_, err := readMirrorFile("/nonexistent/path/mirror.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestReadMirrorFile_InvalidYAML verifies that readMirrorFile returns
// an error when the file contains invalid YAML.
func TestReadMirrorFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mirror.yml")
	if err := os.WriteFile(path, []byte("invalid: yaml: ["), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := readMirrorFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

// TestScopeNames_EmptyMirror verifies that scopeNames returns an empty
// slice for a mirror with no scopes.
func TestScopeNames_EmptyMirror(t *testing.T) {
	m := mirror.Mirror{}
	names := scopeNames(m)
	if len(names) != 0 {
		t.Fatalf("expected empty scope names, got %v", names)
	}
}

// TestScopeNames_NamedScopes verifies that scopeNames returns the correct
// names for a mirror with scopes.
func TestScopeNames_NamedScopes(t *testing.T) {
	m := mirror.Mirror{
		Scopes: []mirror.Scope{
			{Name: "sprint-1", Type: "project", Target: "TEST"},
			{Name: "sprint-2", Type: "project", Target: "TEST"},
		},
	}
	names := scopeNames(m)
	if len(names) != 2 {
		t.Fatalf("expected 2 scope names, got %d", len(names))
	}
	if names[0] != "sprint-1" || names[1] != "sprint-2" {
		t.Fatalf("expected ['sprint-1', 'sprint-2'], got %v", names)
	}
}

// TestBuildStatusSnapshot_EmptyInstance verifies that when the resolved
// project's instance is empty, no credential step is reported.
func TestBuildStatusSnapshot_EmptyInstance(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write settings with no project — empty instance scenario.
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"
credential_refs = ["file://` + filepath.Join(tmpDir, "creds.toml") + `"]

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "creds.toml"), []byte("email = \"user@example.com\"\napi_token = \"token\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile creds: %v", err)
	}
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

	snap := BuildStatusSnapshot(settings, filepath.Join(tmpDir, "mirror"))
	if !snap.Resolved {
		t.Fatal("expected Resolved == true")
	}
	// Since the instance is "default" and it has credentials, no credential step.
	if snap.HasStep("no credentials") {
		t.Fatal("expected no credential step when credentials exist")
	}
}
