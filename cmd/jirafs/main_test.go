package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	settingsDir  = ".jirafs"
	settingsFile = "settings.toml"
)

func TestPrintHelp(t *testing.T) {
	output := runMainHelper(t)
	if !strings.Contains(output.stderr, "Usage:") {
		t.Fatalf("stderr = %q, want help text", output.stderr)
	}
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", output.exitCode)
	}
}

func TestHelpCommand(t *testing.T) {
	output := runMainHelper(t, "help")
	if !strings.Contains(output.stderr, "Commands:") {
		t.Fatalf("stderr = %q, want command list", output.stderr)
	}
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", output.exitCode)
	}
}

func TestMirrorHelpCommand(t *testing.T) {
	output := runMainHelper(t, "mirror", "help")
	if !strings.Contains(output.stderr, "refresh one named live mirror scope") {
		t.Fatalf("stderr = %q, want mirror help text", output.stderr)
	}
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", output.exitCode)
	}
}

func TestShortHelpFlag(t *testing.T) {
	output := runMainHelper(t, "-h")
	if !strings.Contains(output.stderr, "Run \"jirafs <command> --help\"") {
		t.Fatalf("stderr = %q, want help footer", output.stderr)
	}
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", output.exitCode)
	}
}

func TestUnknownCommand(t *testing.T) {
	output := runMainHelper(t, "wat")
	if !strings.Contains(output.stderr, `jirafs: unknown command: "wat"`) {
		t.Fatalf("stderr = %q, want unknown-command message", output.stderr)
	}
	if !strings.Contains(output.stderr, "Usage:") {
		t.Fatalf("stderr = %q, want help text", output.stderr)
	}
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
}

func TestSyncCommand(t *testing.T) {
	output := runMainHelper(t, "sync")
	if !strings.Contains(output.stderr, "jirafs sync: missing issue key") {
		t.Fatalf("stderr = %q, want missing issue key message", output.stderr)
	}
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
}

func TestUnimplementedCommands(t *testing.T) {
	cmds := []string{"init", "new", "registry", "board", "archive"}
	for _, cmd := range cmds {
		output := runMainHelper(t, cmd)
		want := fmt.Sprintf("jirafs %s: not yet implemented", cmd)
		if !strings.Contains(output.stderr, want) {
			t.Fatalf("cmd=%s: stderr = %q, want %q", cmd, output.stderr, want)
		}
		if output.exitCode != 1 {
			t.Fatalf("cmd=%s: exitCode = %d, want 1", cmd, output.exitCode)
		}
	}
}

func TestExportCommand(t *testing.T) {
	output := runMainHelper(t, "export")
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
	if !strings.Contains(output.stderr, "jirafs export: missing subcommand") {
		t.Fatalf("stderr = %q, want missing subcommand message", output.stderr)
	}
}

func TestPlanCommand(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

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

	output := runMainHelperWithHome(t, homeDir, "plan")
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
	if !strings.Contains(output.stderr, "jirafs plan: missing issue key") {
		t.Fatalf("stderr = %q, want missing issue key message", output.stderr)
	}
}

func TestMirrorNoSubcommand(t *testing.T) {
	output := runMainHelper(t, "mirror")
	if !strings.Contains(output.stderr, "jirafs mirror: missing subcommand") {
		t.Fatalf("stderr = %q, want missing subcommand message", output.stderr)
	}
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
}

func TestMirrorUnknownSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

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

	output := runMainHelperWithHome(t, homeDir, "mirror", "unknown-sub")
	if !strings.Contains(output.stderr, `jirafs mirror: unknown subcommand "unknown-sub"`) {
		t.Fatalf("stderr = %q, want unknown subcommand message", output.stderr)
	}
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
}

func TestLongHelpFlag(t *testing.T) {
	output := runMainHelper(t, "--help")
	if !strings.Contains(output.stderr, "Run \"jirafs <command> --help\"") {
		t.Fatalf("stderr = %q, want help footer", output.stderr)
	}
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", output.exitCode)
	}
}

type helperOutput struct {
	stderr   string
	exitCode int
}

func runMainHelper(t *testing.T, args ...string) helperOutput {
	t.Helper()

	cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestMainHelperProcess", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		return helperOutput{stderr: string(output), exitCode: 0}
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("helper process error = %v", err)
	}
	return helperOutput{stderr: string(output), exitCode: exitErr.ExitCode()}
}

func TestUseNoArgsNoProjectSet(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

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

	output := runMainHelperWithHome(t, homeDir, "use")
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0, stderr = %q", output.exitCode, output.stderr)
	}
	if !strings.Contains(output.stderr, "no current project set") {
		t.Fatalf("stderr = %q, want 'no current project set'", output.stderr)
	}
}

func TestUseNoArgsWithProjectSet(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

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
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	output := runMainHelperWithHome(t, homeDir, "use")
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0, stderr = %q", output.exitCode, output.stderr)
	}
	if !strings.Contains(output.stderr, `current project is "platform"`) {
		t.Fatalf("stderr = %q, want 'current project is \"platform\"'", output.stderr)
	}
}

func TestUsePositionalArg(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"

[projects.growth]
key = "GROW"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/growth"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	output := runMainHelperWithHome(t, homeDir, "use", "growth")
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0, stderr = %q", output.exitCode, output.stderr)
	}
	if !strings.Contains(output.stderr, `current project set to "growth"`) {
		t.Fatalf("stderr = %q, want confirmation message", output.stderr)
	}

	// Verify persistence.
	data, err := os.ReadFile(filepath.Join(homeDir, settingsDir, settingsFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "current_project") {
		t.Fatalf("settings = %q, want current_project entry", string(data))
	}
	if !strings.Contains(string(data), "growth") {
		t.Fatalf("settings = %q, want growth project", string(data))
	}
}

func TestUsePositionalArgUnknownProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

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

	output := runMainHelperWithHome(t, homeDir, "use", "UNKNOWN")
	if !strings.Contains(output.stderr, `project "UNKNOWN" not found`) {
		t.Fatalf("stderr = %q, want project not found", output.stderr)
	}
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
}

func TestUseClear(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

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
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	output := runMainHelperWithHome(t, homeDir, "use", "--clear")
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0, stderr = %q", output.exitCode, output.stderr)
	}
	if !strings.Contains(output.stderr, "current project cleared") {
		t.Fatalf("stderr = %q, want 'current project cleared'", output.stderr)
	}

	// Verify persistence: current_project should be empty.
	data, err := os.ReadFile(filepath.Join(homeDir, settingsDir, settingsFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "current_project") {
		t.Fatalf("settings = %q, want current_project entry", string(data))
	}
}

func TestUseClearWithProjectFlag(t *testing.T) {
	output := runMainHelper(t, "use", "--clear", "--project", "PLAT")
	if !strings.Contains(output.stderr, "--project and --clear are mutually exclusive") {
		t.Fatalf("stderr = %q, want mutually exclusive message", output.stderr)
	}
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
}

func TestUseTooManyPositionalArgs(t *testing.T) {
	output := runMainHelper(t, "use", "PLAT", "GROW")
	if !strings.Contains(output.stderr, "too many positional arguments") {
		t.Fatalf("stderr = %q, want too many positional arguments", output.stderr)
	}
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
}

func TestUseUnknownProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

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

	output := runMainHelperWithHome(t, homeDir, "use", "--project", "UNKNOWN")
	if !strings.Contains(output.stderr, `project "UNKNOWN" not found`) {
		t.Fatalf("stderr = %q, want project not found", output.stderr)
	}
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
}

func TestUseUpdatesState(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"

[projects.growth]
key = "GROW"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/growth"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	output := runMainHelperWithHome(t, homeDir, "use", "--project", "growth")
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0, stderr = %q", output.exitCode, output.stderr)
	}
	if !strings.Contains(output.stderr, `current project set to "growth"`) {
		t.Fatalf("stderr = %q, want confirmation message", output.stderr)
	}

	// Verify persistence by reading the file directly.
	data, err := os.ReadFile(filepath.Join(homeDir, settingsDir, settingsFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "current_project") {
		t.Fatalf("settings = %q, want current_project entry", string(data))
	}
	if !strings.Contains(string(data), "growth") {
		t.Fatalf("settings = %q, want growth project", string(data))
	}
}

func TestUseWithExistingState(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := `
version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/platform"

[projects.growth]
key = "GROW"
instance = "work"
mirror_dir = "` + jirafsDir + `/jira/growth"

[state]
current_project = "platform"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	output := runMainHelperWithHome(t, homeDir, "use", "--project", "growth")
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0, stderr = %q", output.exitCode, output.stderr)
	}

	// Verify state was overwritten.
	data, err := os.ReadFile(filepath.Join(homeDir, settingsDir, settingsFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "current_project") {
		t.Fatalf("settings = %q, want current_project entry", string(data))
	}
	if !strings.Contains(string(data), "growth") {
		t.Fatalf("settings = %q, want growth project", string(data))
	}
}

func TestUseMalformedSettings(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a malformed settings file so config.Load() fails.
	malformed := `this is not valid toml {{{`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(malformed), 0o644); err != nil {
		t.Fatal(err)
	}

	output := runMainHelperWithHome(t, homeDir, "use", "PLAT")
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
	if !strings.Contains(output.stderr, "cannot load settings") {
		t.Fatalf("stderr = %q, want 'cannot load settings'", output.stderr)
	}
}

func TestUseInvalidFlag(t *testing.T) {
	output := runMainHelper(t, "use", "--bogus-flag")
	if !strings.Contains(output.stderr, "flag provided but not defined") {
		t.Fatalf("stderr = %q, want 'flag provided but not defined'", output.stderr)
	}
	if output.exitCode != 2 {
		t.Fatalf("exitCode = %d, want 2 (flag.ExitOnError exits 2)", output.exitCode)
	}
}

func TestSetupHelp(t *testing.T) {
	output := runMainHelper(t, "setup", "--help")
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", output.exitCode)
	}
	if !strings.Contains(output.stderr, "Usage of setup") {
		t.Fatalf("stderr = %q, want usage text", output.stderr)
	}
}

func TestSetupMissingFlags(t *testing.T) {
	output := runMainHelper(t, "setup")
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
	if !strings.Contains(output.stderr, "--project is required") {
		t.Fatalf("stderr = %q, want --project required", output.stderr)
	}
}

func TestSetupMissingKey(t *testing.T) {
	output := runMainHelper(t, "setup", "--project", "p")
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
	if !strings.Contains(output.stderr, "--key is required") {
		t.Fatalf("stderr = %q, want --key required", output.stderr)
	}
}

func TestSetupMissingInstance(t *testing.T) {
	output := runMainHelper(t, "setup", "--project", "p", "--key", "K")
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
	if !strings.Contains(output.stderr, "--instance is required") {
		t.Fatalf("stderr = %q, want --instance required", output.stderr)
	}
}

func TestSetupMissingBaseURL(t *testing.T) {
	output := runMainHelper(t, "setup", "--project", "p", "--key", "K", "--instance", "i")
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
	if !strings.Contains(output.stderr, "--base-url is required") {
		t.Fatalf("stderr = %q, want --base-url required", output.stderr)
	}
}

func TestSetupMissingAuthType(t *testing.T) {
	output := runMainHelper(t, "setup", "--project", "p", "--key", "K", "--instance", "i", "--base-url", "https://jira.example.com")
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
	if !strings.Contains(output.stderr, "--auth-type is required") {
		t.Fatalf("stderr = %q, want --auth-type required", output.stderr)
	}
}

func TestSetupMissingMirrorDir(t *testing.T) {
	output := runMainHelper(t, "setup", "--project", "p", "--key", "K", "--instance", "i", "--base-url", "https://j.example.com", "--auth-type", "basic")
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
	if !strings.Contains(output.stderr, "--mirror-dir is required") {
		t.Fatalf("stderr = %q, want --mirror-dir required", output.stderr)
	}
}

func TestSetupInvalidBaseURL(t *testing.T) {
	tmpDir := t.TempDir()
	output := runMainHelper(t, "setup", "--project", "p", "--key", "K", "--instance", "i", "--base-url", "not-a-url", "--auth-type", "basic", "--mirror-dir", filepath.Join(tmpDir, "mirror"))
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
	}
	if !strings.Contains(output.stderr, "--base-url must start with http:// or https://") {
		t.Fatalf("stderr = %q, want invalid URL message", output.stderr)
	}
}

func TestSetupCreatesSettingsFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	output := runMainHelperWithHome(t, homeDir, "setup",
		"--project", "platform",
		"--key", "PLAT",
		"--instance", "work",
		"--base-url", "https://jira.example.com",
		"--auth-type", "atlassian_api_token",
		"--mirror-dir", filepath.Join(tmpDir, "mirror"),
		"--credential-ref", "env://JIRAFS_API_TOKEN",
	)
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0, stderr = %q", output.exitCode, output.stderr)
	}
	if !strings.Contains(output.stderr, "setup complete") {
		t.Fatalf("stderr = %q, want 'setup complete'", output.stderr)
	}

	// Verify the settings file was written.
	data, err := os.ReadFile(filepath.Join(jirafsDir, settingsFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "jira.example.com") {
		t.Errorf("settings = %q, want base_url", text)
	}
	if !strings.Contains(text, "atlassian_api_token") {
		t.Errorf("settings = %q, want auth_type", text)
	}
}

func TestSetupUpdatesExistingSettings(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write an existing settings file.
	existing := `
version = 1

[instances.old]
base_url = "https://old.example.com"
auth_type = "basic"

[projects.legacy]
key = "LEG"
instance = "old"
mirror_dir = "` + filepath.Join(tmpDir, "legacy-mirror") + `"
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run setup to add a new project.
	output := runMainHelperWithHome(t, homeDir, "setup",
		"--project", "newproj",
		"--key", "NP",
		"--instance", "newinst",
		"--base-url", "https://new.example.com",
		"--auth-type", "atlassian_api_token",
		"--mirror-dir", filepath.Join(tmpDir, "new-mirror"),
	)
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0, stderr = %q", output.exitCode, output.stderr)
	}

	// Verify both projects exist in the file.
	data, err := os.ReadFile(filepath.Join(jirafsDir, settingsFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "legacy") {
		t.Errorf("settings = %q, want legacy project preserved", text)
	}
	if !strings.Contains(text, "newproj") {
		t.Errorf("settings = %q, want newproj project", text)
	}
}

func TestSetupFailsOnMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// No settings file exists — setup should create one.
	output := runMainHelperWithHome(t, homeDir, "setup",
		"--project", "p",
		"--key", "K",
		"--instance", "i",
		"--base-url", "https://j.example.com",
		"--auth-type", "basic",
		"--mirror-dir", filepath.Join(tmpDir, "mirror"),
	)
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0, stderr = %q", output.exitCode, output.stderr)
	}
}

func runMainHelperWithHome(t *testing.T, home string, args ...string) helperOutput {
	t.Helper()

	cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestMainHelperProcess", "--"}, args...)...)
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "HOME=") {
			env[i] = "HOME=" + home
			break
		}
	}
	cmd.Env = append(env, "GO_WANT_HELPER_PROCESS=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		return helperOutput{stderr: string(output), exitCode: 0}
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("helper process error = %v", err)
	}
	return helperOutput{stderr: string(output), exitCode: exitErr.ExitCode()}
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
			main()
			return
		}
	}

	main()
}
