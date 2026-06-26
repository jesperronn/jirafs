package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
)

// TestDoctorSnapshot_ZeroValue verifies that a zero-value DoctorSnapshot
// reports IsZero() as true and has no meaningful fields set.
func TestDoctorSnapshot_ZeroValue(t *testing.T) {
	var dsnap DoctorSnapshot
	if !dsnap.IsZero() {
		t.Fatal("zero-value DoctorSnapshot should report IsZero() == true")
	}
	if dsnap.ProjectName != "" || dsnap.ProjectKey != "" ||
		dsnap.MirrorDir != "" || dsnap.Instance != "" ||
		dsnap.Resolved || dsnap.MirrorExists {
		t.Fatal("zero-value DoctorSnapshot should have no meaningful embedded fields")
	}
	if len(dsnap.InstanceCredentials) != 0 {
		t.Fatal("expected no InstanceCredentials")
	}
	if len(dsnap.LiveProbes) != 0 {
		t.Fatal("expected no LiveProbes")
	}
}

// TestBuildDoctorSnapshot_NoSettings verifies that BuildDoctorSnapshot
// with nil settings produces a snapshot with no credentials or probes.
func TestBuildDoctorSnapshot_NoSettings(t *testing.T) {
	dsnap := BuildDoctorSnapshot(nil, "/some/cwd")
	if dsnap.Resolved {
		t.Fatal("expected Resolved == false with nil settings")
	}
	if len(dsnap.InstanceCredentials) != 0 {
		t.Fatalf("expected no InstanceCredentials, got %d", len(dsnap.InstanceCredentials))
	}
	if len(dsnap.LiveProbes) != 0 {
		t.Fatalf("expected no LiveProbes, got %d", len(dsnap.LiveProbes))
	}
	if len(dsnap.MissingSteps) != 1 {
		t.Fatalf("expected 1 missing step, got %d: %v", len(dsnap.MissingSteps), dsnap.MissingSteps)
	}
}

// TestBuildDoctorSnapshot_CredentialResolution verifies that credentials
// are properly resolved and reported in the doctor snapshot.
func TestBuildDoctorSnapshot_CredentialResolution(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write credentials file.
	credsPath := filepath.Join(tmpDir, "creds.toml")
	if err := os.WriteFile(credsPath, []byte("email = \"user@example.com\"\napi_token = \"token\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile creds: %v", err)
	}

	// Write settings with a valid credential ref.
	settingsTOML := `version = 1

[instances.work]
base_url = "https://example.atlassian.net"
auth_type = "atlassian_api_token"
credential_refs = ["file://` + credsPath + `"]

[projects.test]
key = "TEST"
instance = "work"
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

	dsnap := BuildDoctorSnapshot(settings, filepath.Join(tmpDir, "mirror"))

	// Verify credential check for "work" instance.
	cc, ok := dsnap.InstanceCredentials["work"]
	if !ok {
		t.Fatal("expected 'work' in InstanceCredentials")
	}
	if !cc.Resolved {
		t.Fatalf("expected credential resolution to succeed, got: %s", cc.ValidationError)
	}
	if cc.AuthType != "atlassian_api_token" {
		t.Fatalf("expected auth_type 'atlassian_api_token', got %q", cc.AuthType)
	}
	if cc.CredentialSummary == "" {
		t.Fatal("expected non-empty CredentialSummary")
	}

	// Verify that no live probes exist (since we can't actually connect).
	// The live probe should exist but show as not connected since we're
	// using a fake URL.
	lpc, ok := dsnap.LiveProbes["work"]
	if !ok {
		t.Fatal("expected 'work' in LiveProbes")
	}
	if lpc.Connected {
		t.Fatal("expected Connected == false for fake URL")
	}
}

// TestBuildDoctorSnapshot_MissingCredentials verifies that instances with
// no credential_refs report a credential check failure.
func TestBuildDoctorSnapshot_MissingCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write settings WITHOUT credential refs.
	settingsTOML := `version = 1

[instances.work]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "work"
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

	dsnap := BuildDoctorSnapshot(settings, filepath.Join(tmpDir, "mirror"))

	// Verify credential check for "work" instance reports failure.
	cc, ok := dsnap.InstanceCredentials["work"]
	if !ok {
		t.Fatal("expected 'work' in InstanceCredentials")
	}
	if cc.Resolved {
		t.Fatal("expected credential resolution to fail")
	}
	if cc.ValidationError == "" {
		t.Fatal("expected a validation error message")
	}

	// Verify live probe is skipped due to credential failure.
	lpc, ok := dsnap.LiveProbes["work"]
	if !ok {
		t.Fatal("expected 'work' in LiveProbes")
	}
	if lpc.Connected {
		t.Fatal("expected Connected == false")
	}
}

// TestBuildDoctorSnapshot_EmptyInstances verifies that when settings have
// no instances, InstanceCredentials and LiveProbes are empty.
func TestBuildDoctorSnapshot_EmptyInstances(t *testing.T) {
	settings := &config.Settings{
		Version: 1,
		Projects: map[string]config.Project{
			"test": {Key: "TEST", Instance: "work", MirrorDir: "/tmp/mirror", LocalDirs: []string{"/tmp/local"}},
		},
	}

	dsnap := BuildDoctorSnapshot(settings, "/tmp/mirror")
	if len(dsnap.InstanceCredentials) != 0 {
		t.Fatalf("expected no InstanceCredentials, got %d", len(dsnap.InstanceCredentials))
	}
	if len(dsnap.LiveProbes) != 0 {
		t.Fatalf("expected no LiveProbes, got %d", len(dsnap.LiveProbes))
	}
}

// TestDoctorSnapshot_CredentialCheckFields verifies that CredentialCheck
// fields are populated correctly.
func TestDoctorSnapshot_CredentialCheckFields(t *testing.T) {
	cc := CredentialCheck{
		InstanceName:    "work",
		Resolved:        true,
		AuthType:        "atlassian_api_token",
		CredentialSummary: "file (2 field(s))",
	}
	if cc.InstanceName != "work" {
		t.Fatalf("expected InstanceName 'work', got %q", cc.InstanceName)
	}
	if cc.AuthType != "atlassian_api_token" {
		t.Fatalf("expected AuthType 'atlassian_api_token', got %q", cc.AuthType)
	}
	if cc.CredentialSummary != "file (2 field(s))" {
		t.Fatalf("expected CredentialSummary 'file (2 field(s))', got %q", cc.CredentialSummary)
	}
}

// TestDoctorSnapshot_LiveProbeCheckFields verifies that LiveProbeCheck
// fields are populated correctly.
func TestDoctorSnapshot_LiveProbeCheckFields(t *testing.T) {
	lpc := LiveProbeCheck{
		InstanceName:  "work",
		URL:           "https://example.atlassian.net",
		Connected:     true,
		Authenticated: true,
		User:          "Test User",
	}
	if lpc.InstanceName != "work" {
		t.Fatalf("expected InstanceName 'work', got %q", lpc.InstanceName)
	}
	if lpc.URL != "https://example.atlassian.net" {
		t.Fatalf("expected URL 'https://example.atlassian.net', got %q", lpc.URL)
	}
	if !lpc.Connected {
		t.Fatal("expected Connected == true")
	}
	if !lpc.Authenticated {
		t.Fatal("expected Authenticated == true")
	}
	if lpc.User != "Test User" {
		t.Fatalf("expected User 'Test User', got %q", lpc.User)
	}
}

// TestDoctorSnapshot_MultipleInstances verifies that multiple instances
// are all checked for credentials and live-probed.
func TestBuildDoctorSnapshot_MultipleInstances(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write credentials file.
	credsPath := filepath.Join(tmpDir, "creds.toml")
	if err := os.WriteFile(credsPath, []byte("email = \"user@example.com\"\napi_token = \"token\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile creds: %v", err)
	}

	// Write settings with two instances.
	settingsTOML := `version = 1

[instances.work]
base_url = "https://work.atlassian.net"
auth_type = "atlassian_api_token"
credential_refs = ["file://` + credsPath + `"]

[instances.dev]
base_url = "https://dev.atlassian.net"
auth_type = "atlassian_api_token"
credential_refs = ["file://` + credsPath + `"]

[projects.test]
key = "TEST"
instance = "work"
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

	dsnap := BuildDoctorSnapshot(settings, filepath.Join(tmpDir, "mirror"))

	// Both instances should have credential checks.
	if len(dsnap.InstanceCredentials) != 2 {
		t.Fatalf("expected 2 InstanceCredentials, got %d", len(dsnap.InstanceCredentials))
	}
	for _, name := range []string{"work", "dev"} {
		cc, ok := dsnap.InstanceCredentials[name]
		if !ok {
			t.Fatalf("expected %q in InstanceCredentials", name)
		}
		if !cc.Resolved {
			t.Fatalf("expected credential resolution to succeed for %q, got: %s", name, cc.ValidationError)
		}
	}

	// Both instances should have live probes (even if they fail to connect).
	if len(dsnap.LiveProbes) != 2 {
		t.Fatalf("expected 2 LiveProbes, got %d", len(dsnap.LiveProbes))
	}
}

// TestRunDoctor_NoArgs verifies that calling RunDoctor with no arguments
// runs the doctor report (not help) and returns exit code 0.
func TestRunDoctor_NoArgs(t *testing.T) {
	stdout, stderr := withDoctorTestIO(t)
	exit := RunDoctor([]string{})
	if exit != 0 {
		t.Fatalf("RunDoctor([]) = %d, want 0", exit)
	}
	output := stdout.String()
	// Should show doctor sections, not help.
	if !bytes.Contains([]byte(output), []byte("Config:")) {
		t.Fatalf("expected 'Config:' in output, got stdout=%q stderr=%q", output, stderr.String())
	}
	if !bytes.Contains([]byte(output), []byte("Credentials:")) {
		t.Fatalf("expected 'Credentials:' in output, got stdout=%q", output)
	}
	if !bytes.Contains([]byte(output), []byte("Live Probes:")) {
		t.Fatalf("expected 'Live Probes:' in output, got stdout=%q", output)
	}
}

// TestRunDoctor_Help verifies that "--help" returns exit code 0 and prints help.
func TestRunDoctor_Help(t *testing.T) {
	stdout, stderr := withDoctorTestIO(t)
	exit := RunDoctor([]string{"--help"})
	if exit != 0 {
		t.Fatalf("RunDoctor([\"--help\"]) = %d, want 0", exit)
	}
	output := stdout.String() + stderr.String()
	if !bytes.Contains([]byte(output), []byte("doctor")) {
		t.Fatalf("expected 'doctor' in output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

// TestRunDoctor_NoSettings verifies that RunDoctor with no settings file
// reports missing steps and returns exit code 0 (doctor is informational).
func TestRunDoctor_NoSettings(t *testing.T) {
	exit := RunDoctor([]string{})
	if exit != 0 {
		t.Fatalf("RunDoctor([]) = %d, want 0", exit)
	}
}

// TestRunDoctor_FullReport verifies that RunDoctor outputs a full doctor
// report when settings, a project, and credentials are all configured.
func TestRunDoctor_FullReport(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write credentials file.
	credsPath := filepath.Join(tmpDir, "creds.toml")
	if err := os.WriteFile(credsPath, []byte("email = \"user@example.com\"\napi_token = \"token\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile creds: %v", err)
	}

	// Write settings with credentials.
	settingsTOML := `version = 1

[instances.work]
base_url = "https://example.atlassian.net"
auth_type = "atlassian_api_token"
credential_refs = ["file://` + credsPath + `"]

[projects.test]
key = "TEST"
instance = "work"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Write a mirror.yml.
	mirrorDir := filepath.Join(tmpDir, "mirror")
	mirrorYAML := `project:
  type: project
  value: TEST
scopes:
  - name: sprint-1
    type: project
    target: TEST
`
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror.yml: %v", err)
	}

	stdout, stderr := withDoctorTestIO(t)
	exit := RunDoctor([]string{})
	if exit != 0 {
		t.Fatalf("RunDoctor([]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	// Verify all sections are present.
	sections := []string{"Config:", "Credentials:", "Live Probes:", "Onboarding:"}
	for _, section := range sections {
		if !bytes.Contains([]byte(output), []byte(section)) {
			t.Fatalf("expected '%s' in output, got: %s", section, output)
		}
	}
}

// withDoctorTestIO captures stdout and stderr for the doctor command.
func withDoctorTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := doctorStdout
	oldStderr := doctorStderr
	doctorStdout = stdout
	doctorStderr = stderr
	t.Cleanup(func() {
		doctorStdout = oldStdout
		doctorStderr = oldStderr
	})
	return stdout, stderr
}

// TestDoctorSnapshot_EmbeddedStatusSnapshot verifies that the embedded
// StatusSnapshot fields are properly populated in the DoctorSnapshot.
func TestDoctorSnapshot_EmbeddedStatusSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write credentials file.
	credsPath := filepath.Join(tmpDir, "creds.toml")
	if err := os.WriteFile(credsPath, []byte("email = \"user@example.com\"\napi_token = \"token\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile creds: %v", err)
	}

	// Write settings.
	settingsTOML := `version = 1

[instances.work]
base_url = "https://example.atlassian.net"
auth_type = "atlassian_api_token"
credential_refs = ["file://` + credsPath + `"]

[projects.test]
key = "TEST"
instance = "work"
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

	dsnap := BuildDoctorSnapshot(settings, filepath.Join(tmpDir, "mirror"))

	// Verify embedded StatusSnapshot fields.
	if !dsnap.Resolved {
		t.Fatal("expected Resolved == true")
	}
	if dsnap.ProjectName != "test" {
		t.Fatalf("expected ProjectName 'test', got %q", dsnap.ProjectName)
	}
	if dsnap.ProjectKey != "TEST" {
		t.Fatalf("expected ProjectKey 'TEST', got %q", dsnap.ProjectKey)
	}
	if dsnap.Instance != "work" {
		t.Fatalf("expected Instance 'work', got %q", dsnap.Instance)
	}
}
