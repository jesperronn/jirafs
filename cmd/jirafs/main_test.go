package main

import (
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

func TestShortHelpFlag(t *testing.T) {
	output := runMainHelper(t, "-h")
	if !strings.Contains(output.stderr, "Run \"jirafs <command> --help\"") {
		t.Fatalf("stderr = %q, want help footer", output.stderr)
	}
	if output.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", output.exitCode)
	}
}

func TestKnownCommandNotImplemented(t *testing.T) {
	output := runMainHelper(t, "export")
	if !strings.Contains(output.stderr, "jirafs export: not yet implemented") {
		t.Fatalf("stderr = %q, want not-implemented message", output.stderr)
	}
	if output.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", output.exitCode)
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
