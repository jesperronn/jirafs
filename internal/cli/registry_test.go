package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/registry"
)

// withRegistryTestIO swaps registryStdout/registryStderr for the duration of
// the test and restores them on cleanup.
func withRegistryTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := registryStdout
	oldStderr := registryStderr
	registryStdout = stdout
	registryStderr = stderr
	t.Cleanup(func() {
		registryStdout = oldStdout
		registryStderr = oldStderr
	})
	return stdout, stderr
}

// writeRegistrySettings writes a settings.toml with a single project "test"
// (Jira key TEST) whose mirror_dir points at tmpDir/mirror. It returns the
// HOME directory and creates the mirror and local directories.
func writeRegistrySettings(t *testing.T, tmpDir string) string {
	t.Helper()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll jirafs dir: %v", err)
	}
	mirrorDir := filepath.Join(tmpDir, "mirror")
	localDir := filepath.Join(tmpDir, "local")
	for _, dir := range []string{jirafsDir, mirrorDir, localDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + mirrorDir + `"
local_dirs = ["` + localDir + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}
	return homeDir
}

// writeUsersRegistry writes a users registry file with the given entries into
// the test project's mirror directory.
func writeUsersRegistry(t *testing.T, mirrorDir string, entriesYAML string) {
	t.Helper()
	content := "entries:\n" + entriesYAML
	path := filepath.Join(mirrorDir, registry.UsersFile)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

const twoUsersYAML = `  "user:zed":
    display_name: "Zed User"
  "user:amber":
    display_name: "Amber User"
`

func TestRunRegistry_InitUsers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))

	stdout, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"init", "users", "--project", "test"})
	if exit != 0 {
		t.Fatalf("RunRegistry([init users]) = %d, want 0, stderr = %q", exit, stderr.String())
	}

	mirrorDir := filepath.Join(tmpDir, "mirror")
	path := filepath.Join(mirrorDir, registry.UsersFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	if !strings.Contains(string(data), "entries: {}") {
		t.Fatalf("users.yaml = %q, want entries: {}", string(data))
	}

	// The created file must parse back as an empty registry with no error.
	entries, err := registry.LoadUsers(mirrorDir)
	if err != nil {
		t.Fatalf("LoadUsers: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("LoadUsers = %d entries, want 0", len(entries))
	}
	_ = stdout
}

func TestRunRegistry_InitUsers_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))

	mirrorDir := filepath.Join(tmpDir, "mirror")
	writeUsersRegistry(t, mirrorDir, twoUsersYAML)

	_, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"init", "users", "--project", "test"})
	if exit != 0 {
		t.Fatalf("RunRegistry([init users]) on existing = %d, want 0, stderr = %q", exit, stderr.String())
	}
}

func TestRunRegistry_ShowUsers_Human(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))

	mirrorDir := filepath.Join(tmpDir, "mirror")
	writeUsersRegistry(t, mirrorDir, twoUsersYAML)

	stdout, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"show", "users", "--project", "test"})
	if exit != 0 {
		t.Fatalf("RunRegistry([show users]) = %d, want 0, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	// Entries are sorted by ref and rendered as "  <ref>  <summary>".
	amberIdx := strings.Index(output, "  user:amber  Amber User")
	zedIdx := strings.Index(output, "  user:zed  Zed User")
	if amberIdx < 0 || zedIdx < 0 {
		t.Fatalf("show users output = %q, want sorted '  <ref>  <summary>' lines", output)
	}
	if amberIdx > zedIdx {
		t.Fatalf("show users output = %q, want amber before zed (sorted)", output)
	}
}

func TestRunRegistry_ShowUsers_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))

	mirrorDir := filepath.Join(tmpDir, "mirror")
	writeUsersRegistry(t, mirrorDir, twoUsersYAML)

	stdout, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"show", "users", "--json", "--project", "test"})
	if exit != 0 {
		t.Fatalf("RunRegistry([show users --json]) = %d, want 0, stderr = %q", exit, stderr.String())
	}

	var parsed map[string]registry.User
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("show users --json output is not valid JSON: %v\noutput = %q", err, stdout.String())
	}
	if len(parsed) != 2 {
		t.Fatalf("parsed %d entries, want 2", len(parsed))
	}
	if got := parsed["user:amber"].DisplayName; got != "Amber User" {
		t.Fatalf("parsed user:amber.DisplayName = %q, want %q", got, "Amber User")
	}
}

func TestRunRegistry_ShowUsers_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))

	if exit := RunRegistry([]string{"init", "users", "--project", "test"}); exit != 0 {
		t.Fatalf("init users = %d, want 0", exit)
	}

	stdout, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"show", "users", "--project", "test"})
	if exit != 0 {
		t.Fatalf("show users (empty) = %d, want 0, stderr = %q", exit, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "(empty)") {
		t.Fatalf("show users (empty) output = %q, want '(empty)'", output)
	}
}

func TestRunRegistry_ShowUsers_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))

	stdout, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"show", "users", "--project", "test"})
	if exit != 1 {
		t.Fatalf("RunRegistry([show users]) missing = %d, want 1, stdout = %q, stderr = %q", exit, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "registry init users") {
		t.Fatalf("stderr = %q, want 'registry init users' hint", stderr.String())
	}
}

func TestRunRegistry_RefreshUsers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))

	stdout, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"refresh", "users", "--project", "test"})
	if exit != 1 {
		t.Fatalf("RunRegistry([refresh users]) = %d, want 1, stdout = %q, stderr = %q", exit, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "offline") {
		t.Fatalf("stderr = %q, want 'offline' message", stderr.String())
	}
}

func TestRunRegistry_ShowUnknownType(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))

	stdout, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"show", "bogus", "--project", "test"})
	if exit != 1 {
		t.Fatalf("RunRegistry([show bogus]) = %d, want 1, stdout = %q, stderr = %q", exit, stdout.String(), stderr.String())
	}
}

func TestRunRegistry_Help(t *testing.T) {
	stdout, stderr := withRegistryTestIO(t)
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		exit := RunRegistry(args)
		if exit != 0 {
			t.Fatalf("RunRegistry(%v) = %d, want 0", args, exit)
		}
	}
	help := stderr.String()
	for _, want := range []string{"show", "init", "refresh", "--json"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help output missing %q: %q", want, help)
		}
	}
	_ = stdout
}

func TestRunRegistry_UnknownSubcommand(t *testing.T) {
	stdout, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"frobnicate"})
	if exit != 1 {
		t.Fatalf("RunRegistry([frobnicate]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Fatalf("stderr = %q, want 'unknown subcommand'", stderr.String())
	}
	_ = stdout
}

func TestRunRegistry_NoArgs(t *testing.T) {
	stdout, stderr := withRegistryTestIO(t)
	exit := RunRegistry(nil)
	if exit != 1 {
		t.Fatalf("RunRegistry(nil) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Fatalf("stderr = %q, want 'unknown subcommand'", stderr.String())
	}
	_ = stdout
}

// TestRunRegistry_ShowUsers_JSON_FlagsAfterPositional verifies that flags may
// appear after the positional <type> argument, exercising the multi-pass flag
// parser's second iteration.
func TestRunRegistry_ShowUsers_JSON_FlagsAfterPositional(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))

	mirrorDir := filepath.Join(tmpDir, "mirror")
	writeUsersRegistry(t, mirrorDir, twoUsersYAML)

	stdout, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"show", "users", "--json", "--project", "test"})
	if exit != 0 {
		t.Fatalf("RunRegistry([show users --json]) = %d, want 0, stderr = %q", exit, stderr.String())
	}
	var parsed map[string]registry.User
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("show users --json output is not valid JSON: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("parsed %d entries, want 2", len(parsed))
	}
}

// TestRunRegistry_Refresh_MissingType verifies that refresh without a type is
// rejected with a helpful message.
func TestRunRegistry_Refresh_MissingType(t *testing.T) {
	t.Setenv("HOME", writeRegistrySettings(t, t.TempDir()))
	_, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"refresh"})
	if exit != 1 {
		t.Fatalf("RunRegistry([refresh]) = %d, want 1, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "missing <type>") {
		t.Fatalf("stderr = %q, want 'missing <type>'", stderr.String())
	}
}

// TestRunRegistry_Refresh_InvalidFlags verifies that invalid flags are rejected.
func TestRunRegistry_Refresh_InvalidFlags(t *testing.T) {
	t.Setenv("HOME", writeRegistrySettings(t, t.TempDir()))
	_, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"refresh", "--bogus", "users"})
	if exit != 1 {
		t.Fatalf("RunRegistry([refresh --bogus users]) = %d, want 1, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid flags") {
		t.Fatalf("stderr = %q, want 'invalid flags'", stderr.String())
	}
}

// TestRunRegistry_Show_NoSettings verifies that a missing settings file is
// reported as a load error rather than panicking.
func TestRunRegistry_Show_NoSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"show", "users"})
	if exit != 1 {
		t.Fatalf("RunRegistry([show users]) no settings = %d, want 1, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no settings found") {
		t.Fatalf("stderr = %q, want 'no settings found'", stderr.String())
	}
}

// TestRunRegistry_Resolve_NoProject verifies that resolving with no matching
// project yields a "no project resolved" error and lists the candidates.
func TestRunRegistry_Resolve_NoProject(t *testing.T) {
	t.Setenv("HOME", writeRegistrySettings(t, t.TempDir()))
	_, stderr := withRegistryTestIO(t)
	// No --project and the default cwd matches no configured project.
	exit := RunRegistry([]string{"show", "users"})
	if exit != 1 {
		t.Fatalf("RunRegistry([show users]) no project = %d, want 1, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want 'no project resolved'", stderr.String())
	}
	if !strings.Contains(stderr.String(), "test") {
		t.Fatalf("stderr = %q, want candidate project names listed", stderr.String())
	}
}

// TestRunRegistry_Show_UnknownProject verifies that an unknown --project value
// is reported as a resolution error.
func TestRunRegistry_Show_UnknownProject(t *testing.T) {
	t.Setenv("HOME", writeRegistrySettings(t, t.TempDir()))
	_, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"show", "users", "--project", "ghost"})
	if exit != 1 {
		t.Fatalf("RunRegistry([show users --project ghost]) = %d, want 1, stderr = %q", exit, stderr.String())
	}
	if strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want a project-not-found error, not 'no project resolved'", stderr.String())
	}
}

// TestRunRegistry_InitProjects verifies that a second registry type is created
// and exercises the projects branch of registryFileFor.
func TestRunRegistry_InitProjects(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))

	mirrorDir := filepath.Join(tmpDir, "mirror")
	_, stderr := withRegistryTestIO(t)
	exit := RunRegistry([]string{"init", "projects", "--project", "test"})
	if exit != 0 {
		t.Fatalf("RunRegistry([init projects]) = %d, want 0, stderr = %q", exit, stderr.String())
	}
	path := filepath.Join(mirrorDir, registry.ProjectsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	if !strings.Contains(string(data), "entries: {}") {
		t.Fatalf("projects.yaml = %q, want entries: {}", string(data))
	}
}
