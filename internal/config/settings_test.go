package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadValidSettings verifies that a valid settings file loads correctly.
func TestLoadValidSettings(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"
credential_refs = [
  "env://JIRAFS_WORK_API_TOKEN",
]

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"
local_dirs = [
  "` + jirafsDir + `/src/platform-app",
]

[projects.growth]
key = "GROW"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/growth"
local_dirs = []
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}

	if len(s.Instances) != 1 {
		t.Errorf("Instances count = %d, want 1", len(s.Instances))
	}

	if inst, ok := s.Instances["work"]; !ok {
		t.Error("missing instance 'work'")
	} else {
		if inst.BaseURL != "https://jira.example.com" {
			t.Errorf("Instance.work.base_url = %q, want %q", inst.BaseURL, "https://jira.example.com")
		}
		if inst.AuthType != "atlassian_api_token" {
			t.Errorf("Instance.work.auth_type = %q, want %q", inst.AuthType, "atlassian_api_token")
		}
	}

	if len(s.Projects) != 2 {
		t.Errorf("Projects count = %d, want 2", len(s.Projects))
	}

	if proj, ok := s.Projects["platform"]; !ok {
		t.Error("missing project 'platform'")
	} else {
		if proj.Key != "PLAT" {
			t.Errorf("Project.platform.key = %q, want %q", proj.Key, "PLAT")
		}
		if proj.Instance != "work" {
			t.Errorf("Project.platform.instance = %q, want %q", proj.Instance, "work")
		}
		expectedMirror := filepath.Join(jirafsDir, "jira", "platform")
		if proj.MirrorDir != expectedMirror {
			t.Errorf("Project.platform.mirror_dir = %q, want %q", proj.MirrorDir, expectedMirror)
		}
		if len(proj.LocalDirs) != 1 {
			t.Errorf("Project.platform.local_dirs count = %d, want 1", len(proj.LocalDirs))
		}
	}
}

// TestLoadMissingFile verifies that a missing settings file returns an error.
func TestLoadMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing file, got nil")
	}

	if !IsSettingError(err, ErrMissingField) {
		t.Errorf("expected error code %q, got %v", ErrMissingField, err)
	}
}

// TestLoadInvalidTOML verifies that invalid TOML returns a structured error.
func TestLoadInvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	badTOML := `[version
version = 1
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(badTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for invalid TOML, got nil")
	}

	if !IsSettingError(err, ErrMissingField) {
		t.Errorf("expected error code %q, got %v", ErrMissingField, err)
	}
}

// TestLoadMissingVersion verifies that missing version field returns an error.
func TestLoadMissingVersion(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing version, got nil")
	}

	if !IsSettingError(err, ErrMissingField) {
		t.Errorf("expected error code %q, got %v", ErrMissingField, err)
	}
}

// TestLoadMissingBaseURL verifies that missing base_url returns an error.
func TestLoadMissingBaseURL(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
auth_type = "atlassian_api_token"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing base_url, got nil")
	}

	if !IsSettingError(err, ErrMissingField) {
		t.Errorf("expected error code %q, got %v", ErrMissingField, err)
	}
}

// TestLoadInvalidURL verifies that a non-absolute URL returns an error.
func TestLoadInvalidURL(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "jira.example.com"
auth_type = "atlassian_api_token"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for invalid URL, got nil")
	}

	if !IsSettingError(err, ErrInvalidURL) {
		t.Errorf("expected error code %q, got %v", ErrInvalidURL, err)
	}
}

func TestSaveStateWritesSettingsFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := &Settings{
		Version: 1,
		Instances: map[string]Instance{
			"work": {
				BaseURL:  "https://jira.example.com",
				AuthType: "atlassian_api_token",
			},
		},
		Projects: map[string]Project{
			"platform": {
				Key:       "PLAT",
				Instance:  "work",
				MirrorDir: filepath.Join(tmpDir, "mirror"),
			},
		},
		State: State{
			CurrentProject: "platform",
			CurrentUser:    "user:jesper",
		},
	}

	if err := settings.SaveState(); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(jirafsDir, settingsFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	text := string(data)
	if !strings.Contains(text, "current_project = 'platform'") &&
		!strings.Contains(text, "current_project = \"platform\"") {
		t.Fatalf("settings file = %q, want current_project entry", text)
	}
}

func TestSaveStateReturnsStructuredWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := &Settings{
		Version: 1,
		Instances: map[string]Instance{
			"work": {
				BaseURL:  "https://jira.example.com",
				AuthType: "atlassian_api_token",
			},
		},
		Projects: map[string]Project{
			"platform": {
				Key:       "PLAT",
				Instance:  "work",
				MirrorDir: filepath.Join(tmpDir, "mirror"),
			},
		},
		State: State{CurrentProject: "platform"},
	}

	err := settings.SaveState()
	if err == nil {
		t.Fatal("SaveState() error = nil, want write failure")
	}
	if !IsSettingError(err, ErrMissingField) {
		t.Fatalf("SaveState() error = %v, want structured settings error", err)
	}
}

// TestLoadMissingAuthType verifies that missing auth_type returns an error.
func TestLoadMissingAuthType(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing auth_type, got nil")
	}

	if !IsSettingError(err, ErrMissingField) {
		t.Errorf("expected error code %q, got %v", ErrMissingField, err)
	}
}

// TestLoadUnknownInstance verifies that a project referencing a non-existent instance fails.
func TestLoadUnknownInstance(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "nonexistent"
mirror_dir = "` + jirafsDir + `/jira/platform"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for unknown instance, got nil")
	}

	if !IsSettingError(err, ErrUnknownInstance) {
		t.Errorf("expected error code %q, got %v", ErrUnknownInstance, err)
	}
}

// TestLoadUnknownProjectState verifies that state.current_project referencing a non-existent project fails.
func TestLoadUnknownProjectState(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"

[state]
current_project = "nonexistent"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for unknown project in state, got nil")
	}

	if !IsSettingError(err, ErrUnknownProject) {
		t.Errorf("expected error code %q, got %v", ErrUnknownProject, err)
	}
}

// TestLoadMissingProjectKey verifies that missing project key returns an error.
func TestLoadMissingProjectKey(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing project key, got nil")
	}

	if !IsSettingError(err, ErrMissingField) {
		t.Errorf("expected error code %q, got %v", ErrMissingField, err)
	}
}

// TestLoadMissingProjectInstance verifies that missing project instance returns an error.
func TestLoadMissingProjectInstance(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
mirror_dir = "` + jirafsDir + `/jira/platform"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing project instance, got nil")
	}

	if !IsSettingError(err, ErrMissingField) {
		t.Errorf("expected error code %q, got %v", ErrMissingField, err)
	}
}

// TestLoadMissingMirrorDir verifies that missing mirror_dir returns an error.
func TestLoadMissingMirrorDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing mirror_dir, got nil")
	}

	if !IsSettingError(err, ErrMissingField) {
		t.Errorf("expected error code %q, got %v", ErrMissingField, err)
	}
}

// TestLoadNoInstances verifies that no instances returns an error.
func TestLoadNoInstances(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for no instances, got nil")
	}

	if !IsSettingError(err, ErrMissingField) {
		t.Errorf("expected error code %q, got %v", ErrMissingField, err)
	}
}

// TestLoadNoProjects verifies that no projects returns an error.
func TestLoadNoProjects(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for no projects, got nil")
	}

	if !IsSettingError(err, ErrMissingField) {
		t.Errorf("expected error code %q, got %v", ErrMissingField, err)
	}
}

// TestLoadMultipleInstances verifies loading multiple instances works.
func TestLoadMultipleInstances(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[instances.client_a]
base_url = "https://client-a.atlassian.net"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"

[projects.portal]
key = "PORTAL"
instance = "client_a"
mirror_dir = "` + jirafsDir + `/jira/portal"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(s.Instances) != 2 {
		t.Errorf("Instances count = %d, want 2", len(s.Instances))
	}

	if _, ok := s.Instances["work"]; !ok {
		t.Error("missing instance 'work'")
	}
	if _, ok := s.Instances["client_a"]; !ok {
		t.Error("missing instance 'client_a'")
	}
}

// TestLoadStateFields verifies that state fields are loaded correctly.
func TestLoadStateFields(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"

[state]
current_project = "platform"
current_user = "jesper"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if s.State.CurrentProject != "platform" {
		t.Errorf("State.CurrentProject = %q, want %q", s.State.CurrentProject, "platform")
	}
	if s.State.CurrentUser != "jesper" {
		t.Errorf("State.CurrentUser = %q, want %q", s.State.CurrentUser, "jesper")
	}
}

// TestLoadStateNoCurrentProject verifies that missing current_project is OK.
func TestLoadStateNoCurrentProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if s.State.CurrentProject != "" {
		t.Errorf("State.CurrentProject = %q, want empty", s.State.CurrentProject)
	}
}

// TestLoadPathExpansionTilde verifies tilde expansion in mirror_dir and local_dirs.
func TestLoadPathExpansionTilde(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "~/jira/platform"
local_dirs = [
  "~/src/platform-app",
]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	expectedMirror := filepath.Join(homeDir, "jira", "platform")
	if s.Projects["platform"].MirrorDir != expectedMirror {
		t.Errorf("MirrorDir = %q, want %q", s.Projects["platform"].MirrorDir, expectedMirror)
	}

	expectedLocal := filepath.Join(homeDir, "src", "platform-app")
	if len(s.Projects["platform"].LocalDirs) != 1 {
		t.Fatalf("LocalDirs count = %d, want 1", len(s.Projects["platform"].LocalDirs))
	}
	if s.Projects["platform"].LocalDirs[0] != expectedLocal {
		t.Errorf("LocalDirs[0] = %q, want %q", s.Projects["platform"].LocalDirs[0], expectedLocal)
	}
}

// TestLoadPathExpansionEnvVar verifies environment variable expansion in paths.
func TestLoadPathExpansionEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)
	t.Setenv("JIRAFS_MIRROR", "env-expanded-mirror")

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "$JIRAFS_MIRROR/platform"
local_dirs = []
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Env var expands to its literal value; no tilde resolution.
	expectedMirror := "env-expanded-mirror/platform"
	if s.Projects["platform"].MirrorDir != expectedMirror {
		t.Errorf("MirrorDir = %q, want %q", s.Projects["platform"].MirrorDir, expectedMirror)
	}
}

// TestLoadCredentialRefs verifies credential_refs are loaded.
func TestLoadCredentialRefs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"
credential_refs = [
  "file://~/.jirafs/credentials/work-user.toml",
  "env://JIRAFS_WORK_API_TOKEN",
]

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	refs := s.Instances["work"].CredentialRefs
	if len(refs) != 2 {
		t.Fatalf("CredentialRefs count = %d, want 2", len(refs))
	}
	if refs[0] != "file://~/.jirafs/credentials/work-user.toml" {
		t.Errorf("CredentialRefs[0] = %q, want %q", refs[0], "file://~/.jirafs/credentials/work-user.toml")
	}
	if refs[1] != "env://JIRAFS_WORK_API_TOKEN" {
		t.Errorf("CredentialRefs[1] = %q, want %q", refs[1], "env://JIRAFS_WORK_API_TOKEN")
	}
}

// TestLoadDuplicateProjectKey verifies that two projects with the same key are rejected.
func TestLoadDuplicateProjectKey(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"

[projects.portal]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/portal"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for duplicate project key, got nil")
	}

	if !IsSettingError(err, ErrDuplicateProjectKey) {
		t.Errorf("expected error code %q, got %v", ErrDuplicateProjectKey, err)
	}
}

// TestLoadDuplicateMirrorDir verifies that two projects with the same mirror_dir are rejected.
func TestLoadDuplicateMirrorDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/shared"

[projects.portal]
key = "PORTAL"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/shared"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for duplicate mirror_dir, got nil")
	}

	if !IsSettingError(err, ErrDuplicateMirrorDir) {
		t.Errorf("expected error code %q, got %v", ErrDuplicateMirrorDir, err)
	}
}

// TestLoadDuplicateLocalDir verifies that two projects sharing a local_dir are rejected.
func TestLoadDuplicateLocalDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"
local_dirs = [
  "` + jirafsDir + `/shared-src",
]

[projects.portal]
key = "PORTAL"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/portal"
local_dirs = [
  "` + jirafsDir + `/shared-src",
]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for duplicate local_dir, got nil")
	}

	if !IsSettingError(err, ErrDuplicateLocalDir) {
		t.Errorf("expected error code %q, got %v", ErrDuplicateLocalDir, err)
	}
}

// TestLoadDuplicateLocalDirWithinProject verifies duplicate local_dirs within the same project are rejected.
func TestLoadDuplicateLocalDirWithinProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"
local_dirs = [
  "` + jirafsDir + `/src",
  "` + jirafsDir + `/src",
]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for duplicate local_dir within project, got nil")
	}

	if !IsSettingError(err, ErrDuplicateLocalDir) {
		t.Errorf("expected error code %q, got %v", ErrDuplicateLocalDir, err)
	}
}

// TestSetupProjectCreatesNewFile verifies that SetupProject creates a
// settings file from scratch when none exists.
func TestSetupProjectCreatesNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	s := &Settings{
		Version:   1,
		Instances: make(map[string]Instance),
		Projects:  make(map[string]Project),
	}

	err := s.SetupProject("work", "platform", "https://jira.example.com", "atlassian_api_token", filepath.Join(tmpDir, "mirror"), []string{"env://API_TOKEN"})
	if err != nil {
		t.Fatalf("SetupProject() error = %v", err)
	}

	// Verify the file was written.
	data, err := os.ReadFile(filepath.Join(jirafsDir, settingsFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "jira.example.com") {
		t.Errorf("settings = %q, want base_url entry", text)
	}
	if !strings.Contains(text, "atlassian_api_token") {
		t.Errorf("settings = %q, want auth_type entry", text)
	}
	if !strings.Contains(text, "project") {
		t.Errorf("settings = %q, want project section", text)
	}
}

// TestSetupProjectUpdatesExistingInstance verifies that when an instance
// already exists, its base_url and auth_type are overwritten and
// credential_refs are appended.
func TestSetupProjectUpdatesExistingInstance(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	// Write an existing settings file with one instance.
	existing := `
version = 1

[instances.work]
base_url = "https://old.example.com"
auth_type = "basic"
credential_refs = [
  "env://OLD_TOKEN",
]

[projects.legacy]
key = "LEG"
instance = "work"
mirror_dir = "` + filepath.Join(tmpDir, "legacy-mirror") + `"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Now setup a new project on the same instance.
	err = s.SetupProject("work", "newproj", "https://new.example.com", "atlassian_api_token", filepath.Join(tmpDir, "new-mirror"), []string{"env://NEW_TOKEN"})
	if err != nil {
		t.Fatalf("SetupProject() error = %v", err)
	}

	// Reload and verify.
	s2, err := Load()
	if err != nil {
		t.Fatalf("Reload Load() error = %v", err)
	}

	// Instance should be updated.
	inst := s2.Instances["work"]
	if inst.BaseURL != "https://new.example.com" {
		t.Errorf("Instance.work.base_url = %q, want %q", inst.BaseURL, "https://new.example.com")
	}
	if inst.AuthType != "atlassian_api_token" {
		t.Errorf("Instance.work.auth_type = %q, want %q", inst.AuthType, "atlassian_api_token")
	}
	// Credential refs should be appended (2 total: 1 old + 1 new).
	if len(inst.CredentialRefs) != 2 {
		t.Errorf("CredentialRefs count = %d, want 2", len(inst.CredentialRefs))
	}

	// Both projects should exist.
	if _, ok := s2.Projects["legacy"]; !ok {
		t.Error("missing project 'legacy'")
	}
	if _, ok := s2.Projects["newproj"]; !ok {
		t.Error("missing project 'newproj'")
	}
}

// TestSetupProjectUpdatesExistingProject verifies that when a project
// already exists, its fields are overwritten.
func TestSetupProjectUpdatesExistingProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	existing := `
version = 1

[instances.work]
base_url = "https://old.example.com"
auth_type = "basic"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + filepath.Join(tmpDir, "old-mirror") + `"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	err = s.SetupProject("work", "platform", "https://new.example.com", "atlassian_api_token", filepath.Join(tmpDir, "new-mirror"), nil)
	if err != nil {
		t.Fatalf("SetupProject() error = %v", err)
	}

	s2, err := Load()
	if err != nil {
		t.Fatalf("Reload Load() error = %v", err)
	}

	proj := s2.Projects["platform"]
	if proj.MirrorDir != filepath.Join(tmpDir, "new-mirror") {
		t.Errorf("Project.platform.mirror_dir = %q, want %q", proj.MirrorDir, filepath.Join(tmpDir, "new-mirror"))
	}
	if proj.Instance != "work" {
		t.Errorf("Project.platform.instance = %q, want %q", proj.Instance, "work")
	}
}

// TestSetupProjectFailsOnInvalidURL verifies that SetupProject returns
// an error when the provided base_url is not a valid absolute URL.
func TestSetupProjectFailsOnInvalidURL(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	s := &Settings{
		Version:   1,
		Instances: make(map[string]Instance),
		Projects:  make(map[string]Project),
	}

	err := s.SetupProject("work", "platform", "not-a-url", "basic", filepath.Join(tmpDir, "mirror"), nil)
	if err == nil {
		t.Fatal("SetupProject() expected error for invalid URL, got nil")
	}
	if !IsSettingError(err, ErrInvalidURL) {
		t.Errorf("expected error code %q, got %v", ErrInvalidURL, err)
	}
}

// TestSetupProjectCreatesInstanceFirst verifies that SetupProject
// creates the instance before validating the project reference,
// so a project can reference an instance that doesn't yet exist.
func TestSetupProjectCreatesInstanceFirst(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	s := &Settings{
		Version:   1,
		Instances: make(map[string]Instance),
		Projects:  make(map[string]Project),
	}

	// SetupProject creates the instance first, then validates.
	// So referencing a non-existent instance name is fine —
	// the instance gets created as part of the same operation.
	err := s.SetupProject("newinst", "platform", "https://jira.example.com", "basic", filepath.Join(tmpDir, "mirror"), nil)
	if err != nil {
		t.Fatalf("SetupProject() error = %v", err)
	}

	// Verify the instance was created.
	data, err := os.ReadFile(filepath.Join(jirafsDir, settingsFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "newinst") {
		t.Errorf("settings = %q, want instance 'newinst'", text)
	}
}

// TestSetupProjectFailsOnDuplicateMirrorDir verifies that SetupProject
// returns an error when the new mirror_dir duplicates an existing one.
func TestSetupProjectFailsOnDuplicateMirrorDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	mirrorDir := filepath.Join(tmpDir, "shared-mirror")
	existing := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "basic"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + mirrorDir + `"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Try to create a second project with the same mirror_dir.
	err = s.SetupProject("work", "other", "https://other.example.com", "basic", mirrorDir, nil)
	if err == nil {
		t.Fatal("SetupProject() expected error for duplicate mirror_dir, got nil")
	}
	if !IsSettingError(err, ErrDuplicateMirrorDir) {
		t.Errorf("expected error code %q, got %v", ErrDuplicateMirrorDir, err)
	}
}

// TestSetupProjectNoFileCreatesMinimalSettings verifies that calling
// SetupProject with no existing settings file creates a minimal valid
// settings document with version 1.
func TestSetupProjectNoFileCreatesMinimalSettings(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	s := &Settings{}

	err := s.SetupProject("inst", "proj", "https://jira.example.com", "basic", filepath.Join(tmpDir, "mirror"), nil)
	if err != nil {
		t.Fatalf("SetupProject() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(jirafsDir, settingsFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "version = 1") {
		t.Errorf("settings = %q, want version = 1", text)
	}
}
