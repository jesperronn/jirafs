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

// useInitHome points HOME (and XDG_CONFIG_HOME) at a fresh temp dir with no
// settings and returns it.
func useInitHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	return home
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

func initArgs(mirror string, extra ...string) []string {
	return append([]string{
		"platform",
		"--key", "PLAT",
		"--instance", "work",
		"--base-url", "https://jira.example.com",
		"--mirror-dir", mirror,
	}, extra...)
}

func TestRunInit_HappyPath(t *testing.T) {
	home := useInitHome(t)
	tmpDir := t.TempDir()
	stdout, stderr := withInitTestIO(t)

	exit := RunInit(initArgs(tmpDir, "--credential-ref", "env://JIRAFS_API_TOKEN"))
	if exit != 0 {
		t.Fatalf("RunInit = %d, want 0, stderr = %q", exit, stderr.String())
	}
	if out := stdout.String(); !strings.Contains(out, "created: 7") || !strings.Contains(out, filepath.Join(tmpDir, "mirror.yml")) {
		t.Fatalf("expected summary with 'created: 7' and paths on stdout, got %q", out)
	}
	if !strings.Contains(stderr.String(), "writing settings") {
		t.Fatalf("expected progress on stderr, got %q", stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "mirror.yml"))
	if err != nil {
		t.Fatalf("read mirror.yml: %v", err)
	}
	if string(data) != wantMirrorYAML {
		t.Fatalf("mirror.yml mismatch:\n got: %q\nwant: %q", data, wantMirrorYAML)
	}
	for _, name := range []string{
		registry.UsersFile, registry.ProjectsFile, registry.StatusesFile,
		registry.SprintsFile, registry.FixVersionsFile, registry.IssueTypesFile,
	} {
		d, err := os.ReadFile(filepath.Join(tmpDir, name))
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

	settings, err := os.ReadFile(filepath.Join(home, ".jirafs", "settings.toml"))
	if err != nil {
		t.Fatalf("settings not written: %v", err)
	}
	for _, want := range []string{"jira.example.com", "PLAT", "atlassian_api_token", "env://JIRAFS_API_TOKEN"} {
		if !strings.Contains(string(settings), want) {
			t.Fatalf("settings = %q, want %q", settings, want)
		}
	}
	if strings.Contains(string(settings), "current_project = 'platform'") {
		t.Fatalf("current_project must not be set without --set-current: %q", settings)
	}
}

func TestRunInit_Idempotent(t *testing.T) {
	useInitHome(t)
	tmpDir := t.TempDir()
	stdout, stderr := withInitTestIO(t)

	if exit := RunInit(initArgs(tmpDir)); exit != 0 {
		t.Fatalf("first RunInit = %d, stderr = %q", exit, stderr.String())
	}
	custom := []byte("project:\n  type: project\n  value: CUSTOM\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "mirror.yml"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if exit := RunInit(initArgs(tmpDir)); exit != 0 {
		t.Fatalf("second RunInit = %d, stderr = %q", exit, stderr.String())
	}
	if out := stdout.String(); !strings.Contains(out, "created: 0, skipped: 7") {
		t.Fatalf("expected everything skipped, got %q", out)
	}
	got, _ := os.ReadFile(filepath.Join(tmpDir, "mirror.yml"))
	if string(got) != string(custom) {
		t.Fatalf("mirror.yml was overwritten: %q", got)
	}
}

func TestRunInit_RespectsMirrorYaml(t *testing.T) {
	useInitHome(t)
	tmpDir := t.TempDir()
	withInitTestIO(t)
	if err := os.WriteFile(filepath.Join(tmpDir, "mirror.yaml"), []byte("x: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if exit := RunInit(initArgs(tmpDir)); exit != 0 {
		t.Fatalf("RunInit = %d", exit)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "mirror.yml")); !os.IsNotExist(err) {
		t.Fatalf("mirror.yml must not be created next to mirror.yaml, stat err = %v", err)
	}
}

func TestRunInit_SetCurrentAndUpdate(t *testing.T) {
	home := useInitHome(t)
	withInitTestIO(t)
	if exit := RunInit(initArgs(t.TempDir())); exit != 0 {
		t.Fatalf("first RunInit = %d", exit)
	}
	// A second project on existing settings keeps the first and sets current.
	other := []string{"growth", "--key", "GROW", "--instance", "work", "--base-url", "https://jira.example.com", "--mirror-dir", t.TempDir(), "--set-current"}
	if exit := RunInit(other); exit != 0 {
		t.Fatalf("second RunInit = %d", exit)
	}
	t.Setenv("HOME", home)
	s, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if _, ok := s.Projects["platform"]; !ok {
		t.Fatalf("existing project lost: %v", s.Projects)
	}
	if s.State.CurrentProject != "growth" {
		t.Fatalf("current project = %q, want growth", s.State.CurrentProject)
	}
}

func TestRunInit_JiraURLFlagRemoved(t *testing.T) {
	removedFlag := "--jira" + "-url"
	useInitHome(t)
	_, stderr := withInitTestIO(t)
	exit := RunInit([]string{"platform", "--key", "PLAT", "--instance", "work", removedFlag, "https://jira.example.com", "--mirror-dir", t.TempDir()})
	if exit != 2 || !strings.Contains(stderr.String(), "flag provided but not defined: -"+removedFlag[2:]+"") {
		t.Fatalf("exit = %d stderr = %q, want unknown flag usage error", exit, stderr.String())
	}
}

func TestRunInit_ProjectFlagRemoved(t *testing.T) {
	useInitHome(t)
	_, stderr := withInitTestIO(t)
	exit := RunInit([]string{"--project", "platform"})
	if exit != 2 || !strings.Contains(stderr.String(), "flag provided but not defined: -project") {
		t.Fatalf("exit = %d stderr = %q, want unknown flag usage error", exit, stderr.String())
	}
}

func TestRunInit_InvalidBaseURL(t *testing.T) {
	useInitHome(t)
	tmpDir := t.TempDir()
	_, stderr := withInitTestIO(t)
	args := initArgs(tmpDir)
	args[len(args)-3] = "ftp://jira.example.com"
	exit := RunInit(args)
	if exit != 2 || !strings.Contains(stderr.String(), "--base-url must start with http:// or https://") {
		t.Fatalf("exit = %d stderr = %q", exit, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "mirror.yml")); !os.IsNotExist(err) {
		t.Fatalf("expected no mirror.yml for invalid URL, stat err = %v", err)
	}
}

func TestRunInit_MissingValues(t *testing.T) {
	useInitHome(t)
	_, stderr := withInitTestIO(t)
	exit := RunInit([]string{"--mirror-dir", t.TempDir()})
	if exit != 2 {
		t.Fatalf("RunInit = %d, want 2, stderr = %q", exit, stderr.String())
	}
	for _, want := range []string{"<project-name>", "--key", "--instance", "--base-url"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func TestRunInit_TooManyPositionals(t *testing.T) {
	useInitHome(t)
	_, stderr := withInitTestIO(t)
	if exit := RunInit([]string{"a", "b"}); exit != 2 || !strings.Contains(stderr.String(), "too many positional") {
		t.Fatalf("exit = %d stderr = %q", exit, stderr.String())
	}
}

func TestRunInit_DefaultMirrorDirAndAuth(t *testing.T) {
	home := useInitHome(t)
	_, stderr := withInitTestIO(t)
	exit := RunInit([]string{"platform", "--key", "PLAT", "--instance", "work", "--base-url", "https://jira.example.com"})
	if exit != 0 {
		t.Fatalf("RunInit = %d, stderr = %q", exit, stderr.String())
	}
	for _, want := range []string{`default --auth-type="atlassian_api_token"`, `default --mirror-dir="~/jira/platform"`} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	for _, name := range []string{"mirror.yml", registry.UsersFile} {
		if _, err := os.Stat(filepath.Join(home, "jira", "platform", name)); err != nil {
			t.Fatalf("expected %s in default mirror dir: %v", name, err)
		}
	}
}

func TestRunInit_MalformedSettingsFails(t *testing.T) {
	home := useInitHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".jirafs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".jirafs", "settings.toml"), []byte("not = [valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr := withInitTestIO(t)
	if exit := RunInit(initArgs(t.TempDir())); exit != 1 || !strings.Contains(stderr.String(), "cannot load settings") {
		t.Fatalf("exit = %d stderr = %q", exit, stderr.String())
	}
}

func TestRunInit_UncreatableMirrorDir(t *testing.T) {
	useInitHome(t)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr := withInitTestIO(t)
	if exit := RunInit(initArgs(filepath.Join(blocker, "mirror"))); exit != 1 || !strings.Contains(stderr.String(), "cannot create mirror directory") {
		t.Fatalf("exit = %d stderr = %q", exit, stderr.String())
	}
}

func TestRunInit_Help(t *testing.T) {
	stdout, stderr := withInitTestIO(t)
	for _, args := range [][]string{{"--help"}, {"help"}} {
		stdout.Reset()
		if exit := RunInit(args); exit != 0 {
			t.Fatalf("RunInit(%v) = %d, stderr = %q", args, exit, stderr.String())
		}
		if !strings.Contains(stdout.String(), "jirafs init") || !strings.Contains(stdout.String(), "--base-url") || stderr.Len() != 0 {
			t.Fatalf("expected usage on stdout only, got stdout %q stderr %q", stdout.String(), stderr.String())
		}
	}
}
