package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
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
