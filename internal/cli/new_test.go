package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/codec"
)

// withNewTestIO swaps newStdout/newStderr for buffers and restores them on
// cleanup, returning the two buffers for assertions.
func withNewTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	oldOut, oldErr := newStdout, newStderr
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	newStdout = out
	newStderr = errBuf
	t.Cleanup(func() {
		newStdout = oldOut
		newStderr = oldErr
	})
	return out, errBuf
}

// setupNewTest writes a temp HOME with a single "test" project (key TEST) whose
// local_dirs points at a fresh temp dir, and returns that local directory.
func setupNewTest(t *testing.T) string {
	t.Helper()
	localDir := t.TempDir()
	mirrorDir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	jirafsDir := filepath.Join(home, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", jirafsDir, err)
	}

	settings := `version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.test]
key = "TEST"
instance = "work"
mirror_dir = "` + mirrorDir + `"
local_dirs = ["` + localDir + `"]

[state]
current_project = "test"
`
	settingsPath := filepath.Join(jirafsDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte(settings), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	return localDir
}

// firstMarkdownFile returns the path of the single .md file in dir.
func firstMarkdownFile(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			return filepath.Join(dir, e.Name())
		}
	}
	t.Fatalf("no .md file in %s", dir)
	return ""
}

func TestRunNewStory(t *testing.T) {
	localDir := setupNewTest(t)
	out, errBuf := withNewTestIO(t)

	code := RunNew([]string{"story", "--summary", "Export fails on stale refs", "--cwd", localDir})
	if code != 0 {
		t.Fatalf("RunNew exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "created") {
		t.Fatalf("stdout = %q, want it to mention created", out.String())
	}

	path := firstMarkdownFile(t, localDir)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}

	issue, pe := codec.ParseIssue(string(data))
	if pe != nil {
		t.Fatalf("re-parse created issue: %v", pe)
	}
	if issue.Summary != "Export fails on stale refs" {
		t.Errorf("Summary = %q, want %q", issue.Summary, "Export fails on stale refs")
	}
	if issue.Identity.Type != "story" {
		t.Errorf("Identity.Type = %q, want story", issue.Identity.Type)
	}
	if issue.RemoteMetadata.StateFile != "draft" {
		t.Errorf("RemoteMetadata.StateFile = %q, want draft", issue.RemoteMetadata.StateFile)
	}
}

// TestRunNewBug verifies that the bug template produces a draft of type bug.
func TestRunNewBug(t *testing.T) {
	localDir := setupNewTest(t)
	out, errBuf := withNewTestIO(t)

	code := RunNew([]string{"bug", "--summary", "Crash on startup", "--cwd", localDir})
	if code != 0 {
		t.Fatalf("RunNew exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "created") {
		t.Fatalf("stdout = %q, want it to mention created", out.String())
	}

	path := firstMarkdownFile(t, localDir)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}

	issue, pe := codec.ParseIssue(string(data))
	if pe != nil {
		t.Fatalf("re-parse created issue: %v", pe)
	}
	if issue.Identity.Type != "bug" {
		t.Errorf("Identity.Type = %q, want bug", issue.Identity.Type)
	}
	if issue.Summary != "Crash on startup" {
		t.Errorf("Summary = %q, want %q", issue.Summary, "Crash on startup")
	}
	if issue.RemoteMetadata.StateFile != "draft" {
		t.Errorf("RemoteMetadata.StateFile = %q, want draft", issue.RemoteMetadata.StateFile)
	}
}

// TestRunNewEpic verifies that the epic template produces a draft of type epic.
func TestRunNewEpic(t *testing.T) {
	localDir := setupNewTest(t)
	_, errBuf := withNewTestIO(t)

	code := RunNew([]string{"epic", "--summary", "Migrate the datastore", "--cwd", localDir})
	if code != 0 {
		t.Fatalf("RunNew exit = %d, want 0; stderr=%s", code, errBuf.String())
	}

	path := firstMarkdownFile(t, localDir)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}

	issue, pe := codec.ParseIssue(string(data))
	if pe != nil {
		t.Fatalf("re-parse created issue: %v", pe)
	}
	if issue.Identity.Type != "epic" {
		t.Errorf("Identity.Type = %q, want epic", issue.Identity.Type)
	}
}

// TestRunNewUnknownTemplate verifies that an unknown template name is rejected.
func TestRunNewUnknownTemplate(t *testing.T) {
	localDir := setupNewTest(t)
	_, errBuf := withNewTestIO(t)

	code := RunNew([]string{"foobar", "--cwd", localDir})
	if code != 1 {
		t.Fatalf("RunNew unknown template = %d, want 1; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "unknown template") {
		t.Errorf("stderr = %q, want it to mention unknown template", errBuf.String())
	}
}

func TestRunNewEmptySummary(t *testing.T) {
	localDir := setupNewTest(t)
	_, errBuf := withNewTestIO(t)

	tmpl := `---
type: story
project: "project:PROJECT"
schema_version: "1"
state: draft
summary: ""
---

## Description

TODO.
`
	templatePath := filepath.Join(t.TempDir(), "empty.md")
	if err := os.WriteFile(templatePath, []byte(tmpl), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	code := RunNew([]string{"--template", templatePath, "--cwd", localDir})
	if code != 1 {
		t.Fatalf("RunNew exit = %d, want 1; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "summary") {
		t.Fatalf("stderr = %q, want it to mention summary", errBuf.String())
	}
}

func TestRunNewUserTemplate(t *testing.T) {
	localDir := setupNewTest(t)
	_, errBuf := withNewTestIO(t)

	tmpl := `---
type: bug
project: "project:CUSTOM"
schema_version: "1"
state: draft
summary: Custom Bug Report
---

## Description

Custom description.
`
	templatePath := filepath.Join(t.TempDir(), "custom.md")
	if err := os.WriteFile(templatePath, []byte(tmpl), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	code := RunNew([]string{"--template", templatePath, "--cwd", localDir})
	if code != 0 {
		t.Fatalf("RunNew exit = %d, want 0; stderr=%s", code, errBuf.String())
	}

	path := firstMarkdownFile(t, localDir)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}

	issue, pe := codec.ParseIssue(string(data))
	if pe != nil {
		t.Fatalf("re-parse created issue: %v", pe)
	}
	if issue.Identity.Type != "bug" {
		t.Errorf("Identity.Type = %q, want bug", issue.Identity.Type)
	}
	if issue.Summary != "Custom Bug Report" {
		t.Errorf("Summary = %q, want Custom Bug Report", issue.Summary)
	}
}

func TestRunNewMissingSubcommand(t *testing.T) {
	_, errBuf := withNewTestIO(t)

	code := RunNew(nil)
	if code != 2 {
		t.Fatalf("RunNew(nil) exit = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "missing template name") {
		t.Fatalf("stderr = %q, want 'missing template name'", errBuf.String())
	}
}

func TestRunNewHelp(t *testing.T) {
	out, _ := withNewTestIO(t)

	for _, arg := range []string{"help", "--help", "-h"} {
		if code := RunNew([]string{arg}); code != 0 {
			t.Fatalf("RunNew(%q) exit = %d, want 0", arg, code)
		}
	}
	if out.Len() == 0 {
		t.Fatalf("help produced no output")
	}
}
