package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// helpCommands lists every command and subcommand that has its own help page.
var helpCommands = [][]string{
	{},
	{"init"},
	{"export"}, {"export", "issue"},
	{"diff"},
	{"status"},
	{"doctor"},
	{"sync"},
	{"new"},
	{"push"},
	{"registry"},
	{"board"},
	{"archive"}, {"archive", "issue"}, {"archive", "list"},
	{"mirror"}, {"mirror", "refresh"}, {"mirror", "archive-sweep"},
	{"use"},
}

type splitOutput struct {
	stdout, stderr string
	exitCode       int
}

// runSplit re-execs the test binary as jirafs with separate stdout/stderr.
func runSplit(t *testing.T, home string, args ...string) splitOutput {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestMainHelperProcess", "--"}, args...)...)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "HOME="+home, "XDG_CONFIG_HOME="+home)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return splitOutput{stdout: out.String(), stderr: errb.String(), exitCode: code}
}

// TestHelpNeedsNoSettings: explicit help works with an empty HOME, on
// stdout, exit 0, and nothing on stderr.
func TestHelpNeedsNoSettings(t *testing.T) {
	emptyHome := t.TempDir()
	for _, base := range helpCommands {
		for _, flag := range []string{"--help", "-h"} {
			args := append(append([]string{}, base...), flag)
			got := runSplit(t, emptyHome, args...)
			if got.exitCode != 0 || strings.TrimSpace(got.stdout) == "" || got.stderr != "" {
				t.Errorf("%v: exit=%d stdout=%q stderr=%q; want exit 0, help on stdout, empty stderr", args, got.exitCode, got.stdout, got.stderr)
			}
			if !strings.Contains(got.stdout, "Usage:") {
				t.Errorf("%v: stdout has no Usage: %q", args, got.stdout)
			}
		}
	}
	for _, args := range [][]string{{"help"}, {"export", "help"}, {"mirror", "help"}, {"archive", "help"}, {"registry", "help"}} {
		got := runSplit(t, emptyHome, args...)
		if got.exitCode != 0 || got.stdout == "" || got.stderr != "" {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q", args, got.exitCode, got.stdout, got.stderr)
		}
	}
}

// TestUnknownFlagIsUsageError: a bad flag is one error line plus the hint on
// stderr, exit 2, nothing on stdout. Settings exist so flag errors are not
// masked by a missing-settings failure.
func TestUnknownFlagIsUsageError(t *testing.T) {
	home := writeMinimalHome(t)
	for _, base := range helpCommands {
		if len(base) == 0 {
			continue
		}
		args := append(append([]string{}, base...), "--no-such-flag")
		got := runSplit(t, home, args...)
		if got.exitCode != 2 || got.stdout != "" || !strings.Contains(got.stderr, "Use --help for usage.") {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q; want exit 2, empty stdout, usage hint on stderr", args, got.exitCode, got.stdout, got.stderr)
		}
	}
}

// TestUsageErrorsExitTwo covers unknown subcommands and missing arguments.
func TestUsageErrorsExitTwo(t *testing.T) {
	home := writeMinimalHome(t)
	for _, args := range [][]string{
		{"wat"},
		{"export"}, {"export", "wat"}, {"export", "issue"},
		{"mirror"}, {"mirror", "wat"}, {"mirror", "refresh"},
		{"archive"}, {"archive", "wat"}, {"archive", "issue"},
		{"registry"}, {"registry", "wat"}, {"registry", "show"},
		{"new"}, {"push"}, {"sync"},
	} {
		got := runSplit(t, home, args...)
		if got.exitCode != 2 || got.stdout != "" || !strings.Contains(got.stderr, "Use --help for usage.") {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q; want exit 2, empty stdout, usage hint", args, got.exitCode, got.stdout, got.stderr)
		}
	}
}

// TestUnknownSubcommandNeedsNoSettings: usage errors do not depend on settings.
func TestUnknownSubcommandNeedsNoSettings(t *testing.T) {
	got := runSplit(t, t.TempDir(), "export", "wat")
	if got.exitCode != 2 || strings.Contains(got.stderr, "settings") {
		t.Fatalf("exit=%d stderr=%q; want exit 2 without a settings message", got.exitCode, got.stderr)
	}
}

func TestNoTaskIDsInUserVisibleStrings(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("main.go"))
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(data), "\n") {
		for _, id := range []string{"B0", "B1"} {
			if idx := strings.Index(line, id); idx >= 0 && len(line) >= idx+4 && isDigits(line[idx+1:idx+4]) {
				t.Errorf("main.go:%d mentions a task id: %s", i+1, strings.TrimSpace(line))
			}
		}
	}
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
