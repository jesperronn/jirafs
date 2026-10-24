package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/registry"
)

// withInitTestIO swaps the init command's stdout/stderr for buffers and
// restores the originals when the test (or cleanup) finishes.
func withInitTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := initStdout
	oldStderr := initStderr
	initStdout = stdout
	initStderr = stderr
	t.Cleanup(func() {
		initStdout = oldStdout
		initStderr = oldStderr
	})
	return stdout, stderr
}

// chdirTemp moves the process into a fresh temp dir for the duration of the
// test and returns it.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(oldCwd) })
	return dir
}

// wantMirrorYAML is the exact body init writes to mirror.yml for key PLAT.
const wantMirrorYAML = `project:
  type: project
  value: PLAT
scopes:
  - name: current-sprint
    type: jql
    target: sprint in openSprints()
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`

func TestRunInit_HappyPath(t *testing.T) {
	tmpDir := chdirTemp(t)
	stdout, stderr := withInitTestIO(t)

	args := []string{
		"--project", "platform",
		"--key", "PLAT",
		"--instance", "work",
		"--jira-url", "https://jira.example.com",
		"--mirror-dir", tmpDir,
	}

	exit := RunInit(args)
	if exit != 0 {
		t.Fatalf("RunInit = %d, want 0, stderr = %q", exit, stderr.String())
	}

	if out := stdout.String(); !strings.Contains(out, "created: 7") {
		t.Fatalf("expected 'created: 7' on first run, got %q", out)
	}

	// mirror.yml must match the expected shape exactly.
	data, err := os.ReadFile(filepath.Join(tmpDir, "mirror.yml"))
	if err != nil {
		t.Fatalf("read mirror.yml: %v", err)
	}
	if string(data) != wantMirrorYAML {
		t.Fatalf("mirror.yml mismatch:\n got: %q\nwant: %q", data, wantMirrorYAML)
	}

	// All six registry files must exist with the empty-but-valid body and be
	// parseable by the registry loaders.
	for _, name := range []string{
		registry.UsersFile,
		registry.ProjectsFile,
		registry.StatusesFile,
		registry.SprintsFile,
		registry.FixVersionsFile,
		registry.IssueTypesFile,
	} {
		p := filepath.Join(tmpDir, name)
		d, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(d) != registryEntryContent {
			t.Fatalf("%s content = %q, want %q", name, d, registryEntryContent)
		}
	}
	if _, err := registry.LoadUsers(tmpDir); err != nil {
		t.Fatalf("registry.LoadUsers rejected the created file: %v", err)
	}
}

func TestRunInit_Idempotent(t *testing.T) {
	tmpDir := chdirTemp(t)
	_, stderr := withInitTestIO(t)

	args := []string{
		"--project", "platform",
		"--key", "PLAT",
		"--mirror-dir", tmpDir,
	}

	if exit := RunInit(args); exit != 0 {
		t.Fatalf("first RunInit = %d, want 0, stderr = %q", exit, stderr.String())
	}

	stdout, stderr := withInitTestIO(t)
	exit := RunInit(args)
	if exit != 0 {
		t.Fatalf("second RunInit = %d, want 0, stderr = %q", exit, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "created: 0") {
		t.Fatalf("expected 'created: 0' on second run, got %q", out)
	}
	if !strings.Contains(out, "skipped: 7") {
		t.Fatalf("expected 'skipped: 7' on second run, got %q", out)
	}

	// The files must be byte-for-byte unchanged after the second run.
	data, err := os.ReadFile(filepath.Join(tmpDir, "mirror.yml"))
	if err != nil {
		t.Fatalf("read mirror.yml: %v", err)
	}
	if string(data) != wantMirrorYAML {
		t.Fatalf("mirror.yml changed on idempotent run:\n got: %q\nwant: %q", data, wantMirrorYAML)
	}
}

func TestRunInit_SetCurrent(t *testing.T) {
	tmpDir := chdirTemp(t)
	homeDir := filepath.Join(tmpDir, "home")
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withInitTestIO(t)

	args := []string{
		"--project", "platform",
		"--key", "PLAT",
		"--instance", "work",
		"--jira-url", "https://jira.example.com",
		"--mirror-dir", tmpDir,
		"--set-current",
	}

	exit := RunInit(args)
	if exit != 0 {
		t.Fatalf("RunInit = %d, want 0, stderr = %q", exit, stderr.String())
	}

	s, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if s.State.CurrentProject != "platform" {
		t.Fatalf("CurrentProject = %q, want %q", s.State.CurrentProject, "platform")
	}
	if _, ok := s.Projects["platform"]; !ok {
		t.Fatalf("expected project \"platform\" recorded in settings")
	}
	if _, ok := s.Instances["work"]; !ok {
		t.Fatalf("expected instance \"work\" recorded in settings")
	}
}

func TestRunInit_InvalidJiraURL(t *testing.T) {
	tmpDir := chdirTemp(t)
	_, stderr := withInitTestIO(t)

	exit := RunInit([]string{
		"--project", "platform",
		"--jira-url", "ftp://jira.example.com",
		"--mirror-dir", tmpDir,
	})
	if exit != 1 {
		t.Fatalf("RunInit = %d, want 1, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "http") {
		t.Fatalf("expected 'http' hint in stderr, got %q", stderr.String())
	}
	// Validation happens before any layout is written.
	if _, err := os.Stat(filepath.Join(tmpDir, "mirror.yml")); !os.IsNotExist(err) {
		t.Fatalf("expected no mirror.yml for invalid URL, stat err = %v", err)
	}
}

func TestRunInit_MissingProject(t *testing.T) {
	chdirTemp(t)
	homeDir := filepath.Join(t.TempDir(), "home")
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withInitTestIO(t)

	exit := RunInit([]string{"--mirror-dir", t.TempDir()})
	if exit != 1 {
		t.Fatalf("RunInit = %d, want 1, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "run jirafs init --project") {
		t.Fatalf("expected next-step hint in stderr, got %q", stderr.String())
	}
}

func TestRunInit_DefaultMirrorDir(t *testing.T) {
	chdirTemp(t)
	homeDir := filepath.Join(t.TempDir(), "home")
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withInitTestIO(t)

	exit := RunInit([]string{"--project", "platform", "--key", "PLAT"})
	if exit != 0 {
		t.Fatalf("RunInit = %d, want 0, stderr = %q", exit, stderr.String())
	}

	wantDir := filepath.Join(homeDir, "jira", "platform")
	for _, name := range []string{"mirror.yml", registry.UsersFile} {
		if _, err := os.Stat(filepath.Join(wantDir, name)); err != nil {
			t.Fatalf("expected %s in default mirror dir %q: %v", name, wantDir, err)
		}
	}
}

func TestRunInit_Help(t *testing.T) {
	stdout, stderr := withInitTestIO(t)

	exit := RunInit([]string{"--help"})
	if exit != 0 {
		t.Fatalf("RunInit([--help]) = %d, want 0, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: jirafs init") || stderr.Len() != 0 {
		t.Fatalf("expected usage on stdout only, got stdout %q stderr %q", stdout.String(), stderr.String())
	}
}
