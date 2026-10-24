package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

// TestRunPull_Help verifies that help forms return exit code 0 before any
// settings are loaded.
func TestRunPull_Help(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"my-issues", "--help"}, {"PLAT-1", "-h"}} {
		stdout, _ := withPullTestIO(t)
		if exit := RunPull(args); exit != 0 {
			t.Fatalf("RunPull(%v) = %d, want 0", args, exit)
		}
		if !strings.Contains(stdout.String(), "pull [scope|KEY]") {
			t.Fatalf("RunPull(%v) stdout = %q, want pull usage", args, stdout.String())
		}
	}
}

// TestRunPull_BuiltinScope verifies that a built-in scope name pulls without
// any mirror.yml entry.
func TestRunPull_BuiltinScope(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("my-issues", []*schema.Issue{})
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)

	exit := RunPull([]string{"my-issues", "--project", "test"})
	if exit != 0 {
		t.Fatalf("RunPull([\"my-issues\", \"--project\", \"test\"]) = %d, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `no changed issue keys for scope "my-issues"`) {
		t.Fatalf("stdout = %q, want refresh summary", stdout.String())
	}
}

func withPullTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := pullStdout
	oldStderr := pullStderr
	pullStdout = stdout
	pullStderr = stderr
	t.Cleanup(func() {
		pullStdout = oldStdout
		pullStderr = oldStderr
	})
	return stdout, stderr
}

func withPullClientFactory(t *testing.T, factory func(*config.Settings, *context.Context, string) (jira.Client, error)) {
	t.Helper()
	oldFactory := pullClientFactory
	pullClientFactory = factory
	t.Cleanup(func() {
		pullClientFactory = oldFactory
	})
}

// TestRunMirrorArchiveSweep_NoProject verifies that archive-sweep
// returns error when no project can be resolved.
func writeSettings(t *testing.T, tmpDir string) string {
	t.Helper()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	settingsTOML := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(tmpDir, "mirror") + `"
local_dirs = ["` + filepath.Join(tmpDir, "local") + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, "settings.toml"), []byte(settingsTOML), 0o644); err != nil {
		t.Fatalf("WriteFile settings: %v", err)
	}
	return homeDir
}

func writeMirror(t *testing.T, tmpDir string) {
	t.Helper()
	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	mirrorYAML := `project:
  type: project
  value: TEST
`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}
}

func writeMirrorWithScopes(t *testing.T, tmpDir string, scopesYAML string) {
	t.Helper()
	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	mirrorYAML := `project:
  type: project
  value: TEST
` + scopesYAML
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}
}

func writeIssue(t *testing.T, localDir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(localDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
}

func TestRunPull_FlagError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	_, stderr := withPullTestIO(t)
	exit := RunPull([]string{"--bogus"})
	if exit != 2 {
		t.Fatalf("RunPull([\"--bogus\"]) = %d, want 2", exit)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("stderr = %q, want flag error", stderr.String())
	}
}

func TestRunPull_ResolvesProjectAndRefreshesScope(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("my-issues", []*schema.Issue{
		{Identity: schema.IssueIdentity{Key: "TEST-2", Type: "story"}},
		{Identity: schema.IssueIdentity{Key: "TEST-1", Type: "bug"}},
	})
	fake.SetIssue("TEST-1", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "TEST-1",
			Type:    "bug",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"},
		},
		MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
		RemoteMetadata: schema.RemoteMetadata{
			StateFile:     "synced",
			RemoteVersion: "1",
			ContentHash:   "abc",
		},
		Summary: "First refreshed issue",
	})
	fake.SetIssue("TEST-2", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "TEST-2",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"},
		},
		MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
		RemoteMetadata: schema.RemoteMetadata{
			StateFile:     "synced",
			RemoteVersion: "2",
			ContentHash:   "def",
		},
		Summary: "Second refreshed issue",
	})
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--project", "test", "my-issues"})
	if exit != 0 {
		t.Fatalf("RunPull(refresh) = %d, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `2 changed issue key(s) for scope "my-issues"`) {
		t.Fatalf("stdout = %q, want refresh summary", stdout.String())
	}
	if !strings.Contains(stdout.String(), "fetching 0/2") || !strings.Contains(stdout.String(), "fetched 2/2") {
		t.Fatalf("stdout = %q, want progress output", stdout.String())
	}
	first := strings.Index(stdout.String(), "TEST-1")
	second := strings.Index(stdout.String(), "TEST-2")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("stdout = %q, want deterministic sorted key order", stdout.String())
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "mirror", "mirror.yml"))
	if err != nil {
		t.Fatalf("ReadFile mirror.yml: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "scope_members:") {
		t.Fatalf("mirror.yml = %q, want scope_members", got)
	}
	if !strings.Contains(got, "key: TEST-1") || !strings.Contains(got, "key: TEST-2") {
		t.Fatalf("mirror.yml = %q, want both scope member keys", got)
	}

	test1Data, err := os.ReadFile(filepath.Join(tmpDir, "local", "TEST-1.md"))
	if err != nil {
		t.Fatalf("ReadFile TEST-1.md: %v", err)
	}
	if !strings.Contains(string(test1Data), "First refreshed issue") {
		t.Fatalf("TEST-1.md = %q, want refreshed issue summary", string(test1Data))
	}
}

func TestRunPull_ScopeNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	_, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--project", "test", "no-such-scope"})
	if exit != 1 {
		t.Fatalf("RunPull(refresh unknown scope) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), `scope "no-such-scope" not found`) {
		t.Fatalf("stderr = %q, want scope not found", stderr.String())
	}
}

func TestRunPull_UnknownScopeListsConfiguredNames(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: plat
    type: project
    target: PLAT
  - name: urgent
    type: jql
    target: priority = Highest
`)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	_, stderr := withPullTestIO(t)

	if exit := RunPull([]string{"--project", "test", "nope"}); exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	out := stderr.String()
	for _, want := range []string{"plat", "urgent", "my-issues", "current-sprint", "scopes:\n  - name: nope\n    type: jql\n    target: "} {
		if !strings.Contains(out, want) {
			t.Errorf("stderr missing %q:\n%s", want, out)
		}
	}
}

func TestRunPull_ProjectScopeSendsJQL(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: plat
    type: project
    target: PLAT
`)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope(`project = "PLAT" ORDER BY key`, []*schema.Issue{})
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	withPullTestIO(t)

	if exit := RunPull([]string{"--project", "test", "plat"}); exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	got := fake.Searches()
	if len(got) == 0 || got[0] != `project = "PLAT" ORDER BY key` {
		t.Fatalf("searches = %q, want project JQL first", got)
	}
}

func TestRunPull_BuiltinScopeWithoutMirrorEntry(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	withPullTestIO(t)

	if exit := RunPull([]string{"--project", "test", "current-sprint"}); exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if got := fake.Searches(); len(got) == 0 || got[0] != "sprint in openSprints()" {
		t.Fatalf("searches = %q, want built-in sprint JQL", got)
	}
}

func TestPullHelpListsOnlySupportedScopes(t *testing.T) {
	stdout, stderr := withPullTestIO(t)
	printPullHelp()
	out := stdout.String()
	for _, banned := range []string{"backlog", "next-sprint"} {
		if strings.Contains(out, banned) {
			t.Errorf("help mentions unsupported scope %q:\n%s", banned, out)
		}
	}
	for _, want := range []string{"my-issues", "current-sprint"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q", want)
		}
	}
	t.Setenv("HOME", writeSettings(t, t.TempDir()))
	if exit := RunPull([]string{"--project", "test", "backlog"}); exit != 1 || !strings.Contains(stderr.String(), `scope "backlog" not found`) {
		t.Errorf("RunPull(backlog) = %d, stderr = %q, want unknown scope", exit, stderr.String())
	}
}

func TestRunPull_TooManyArgs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	_, stderr := withPullTestIO(t)

	exit := RunPull([]string{"my-issues", "extra"})
	if exit != 2 {
		t.Fatalf("RunPull(refresh too many args) = %d, want 2", exit)
	}
	if !strings.Contains(stderr.String(), "too many positional arguments") {
		t.Fatalf("stderr = %q, want too many positional arguments", stderr.String())
	}
}

func TestRunPull_ClientCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, os.ErrPermission
	})
	_, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--project", "test", "my-issues"})
	if exit != 1 {
		t.Fatalf("RunPull(refresh client error) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create Jira client") {
		t.Fatalf("stderr = %q, want client creation error", stderr.String())
	}
}

func TestRunPull_SearchError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetErr("search", jira.NewHTTPErr(500, "server error"))
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--project", "test", "my-issues"})
	if exit != 1 {
		t.Fatalf("RunPull(refresh search error) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "server error") {
		t.Fatalf("stderr = %q, want search error", stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "mirror", "mirror.yml"))
	if err != nil {
		t.Fatalf("ReadFile mirror.yml: %v", err)
	}
	if strings.Contains(string(data), "scope_members:") {
		t.Fatalf("mirror.yml = %q, want no persisted scope members on error", string(data))
	}
}

func TestRunPull_NoChangedIssueKeys(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: my-issues
    type: jql
    target: assignee = currentUser()
scope_members:
  - key: TEST-1
    scope: my-issues
`)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("my-issues", []*schema.Issue{
		{Identity: schema.IssueIdentity{Key: "TEST-1", Type: "story"}},
	})
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--project", "test", "my-issues"})
	if exit != 0 {
		t.Fatalf("RunPull(refresh no changes) = %d, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `no changed issue keys for scope "my-issues"`) {
		t.Fatalf("stdout = %q, want no changed issue keys message", stdout.String())
	}
}

// TestRunMirrorArchiveSweep_EligibleIssues verifies that archive-sweep
// reports eligible issues without mutation.
// TestRunMirrorArchiveSweep_NoEligibleIssues verifies that archive-sweep
// reports nothing when no issues are eligible.
// TestRunMirrorArchiveSweep_PinnedIssue verifies that pinned issues
// are not reported as eligible.
// TestRunMirrorArchiveSweep_UnsyncedIssue verifies that unsynced issues
// (zero remote metadata) are not reported as eligible.
// TestRunMirrorArchiveSweep_ExplicitImportNotEligible verifies that
// explicitly imported issues are not reported as eligible.
// TestRunMirrorArchiveSweep_ScopeMemberNotEligible verifies that scope
// members are not reported as eligible.
// TestLoadMirrorYAML_NoMirrorFile verifies that loadMirrorYAML returns an
// empty mirror when no mirror file exists.
func TestLoadMirrorYAML_NoMirrorFile(t *testing.T) {
	tmpDir := t.TempDir()
	m, path, err := loadMirrorYAML(tmpDir)
	if err != nil {
		t.Fatalf("loadMirrorYAML: %v", err)
	}
	if m == nil || m.Project.Value != "" {
		t.Errorf("loadMirrorYAML(empty dir) = %#v, want empty mirror", m)
	}
	if path != filepath.Join(tmpDir, "mirror.yml") {
		t.Fatalf("path = %q, want default mirror.yml path", path)
	}
}

// TestLoadMirrorYAML_YamlExtension verifies that loadMirrorYAML finds
// mirror.yaml when mirror.yml does not exist.
func TestLoadMirrorYAML_YamlExtension(t *testing.T) {
	tmpDir := t.TempDir()
	mirrorYAML := `project:
  type: project
  value: TEST
`
	if err := os.WriteFile(filepath.Join(tmpDir, "mirror.yaml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror.yaml: %v", err)
	}
	m, _, err := loadMirrorYAML(tmpDir)
	if err != nil {
		t.Fatalf("loadMirrorYAML: %v", err)
	}
	if m == nil || m.Project.Value != "TEST" {
		t.Errorf("loadMirrorYAML = %#v, want project TEST", m)
	}
}

// TestLoadMirrorYAML_InvalidYAML verifies that loadMirrorYAML returns an
// error when the mirror file contains invalid YAML.
func TestLoadMirrorYAML_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	invalidYAML := `: : invalid {{{ yaml`
	if err := os.WriteFile(filepath.Join(tmpDir, "mirror.yml"), []byte(invalidYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror.yml: %v", err)
	}
	_, _, err := loadMirrorYAML(tmpDir)
	if err == nil {
		t.Fatal("loadMirrorYAML(invalid) = nil, want error")
	}
}

func TestResolveMirrorContext_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	_, stderr := withPullTestIO(t)

	ctx, ok := resolveMirrorContext(settings, "", filepath.Join(tmpDir, "outside"), "refresh")
	if ok {
		t.Fatal("expected unresolved project")
	}
	if ctx != nil {
		t.Fatalf("expected nil context, got %#v", ctx)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Available projects:") {
		t.Fatalf("stderr = %q, want candidate list", stderr.String())
	}
}

func TestBuildMirrorClient_Success(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "creds.toml")
	if err := os.WriteFile(credsPath, []byte("email = \"user@example.com\"\napi_token = \"token\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile creds.toml: %v", err)
	}
	settings := &config.Settings{
		Instances: map[string]config.Instance{
			"default": {
				BaseURL:        "https://example.atlassian.net",
				AuthType:       "atlassian_api_token",
				CredentialRefs: []string{"file://" + credsPath},
			},
		},
	}

	client, err := newJiraClient(settings, &context.Context{Instance: "default"}, ".")
	if err != nil {
		t.Fatalf("newJiraClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestFindIssuePath_Success verifies that findIssuePath finds the correct file.
func TestFindIssuePath_Success(t *testing.T) {
	tmpDir := t.TempDir()
	localDir1 := filepath.Join(tmpDir, "local1")
	localDir2 := filepath.Join(tmpDir, "local2")
	if err := os.MkdirAll(localDir1, 0o755); err != nil {
		t.Fatalf("MkdirAll local1: %v", err)
	}
	if err := os.MkdirAll(localDir2, 0o755); err != nil {
		t.Fatalf("MkdirAll local2: %v", err)
	}

	// Write a file in localDir2.
	writeIssue(t, localDir2, "PROJ-42.md", `---
key: PROJ-42
type: story
project:
  type: project
  value: PROJ
---

Summary: Test issue
`)

	localDirs := []string{localDir1, localDir2}
	path, found := findIssuePath(localDirs, "PROJ-42")
	if !found {
		t.Fatal("expected to find PROJ-42")
	}
	want := filepath.Join(localDir2, "PROJ-42.md")
	if path != want {
		t.Errorf("findIssuePath = %q, want %q", path, want)
	}
}

// TestFindIssuePath_NotFound verifies that findIssuePath returns false
// when the file is not in any local directory.
func TestFindIssuePath_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	_, found := findIssuePath([]string{localDir}, "PROJ-99")
	if found {
		t.Error("expected not found")
	}
}

// TestFindIssuePath_EmptyDirs verifies that findIssuePath returns false
// when given empty local directories.
func TestFindIssuePath_EmptyDirs(t *testing.T) {
	_, found := findIssuePath([]string{}, "PROJ-99")
	if found {
		t.Error("expected not found with empty dirs")
	}
}

// TestRunPull_ProjectFlagNotInSettings verifies that passing
// --project with a project name that does not exist in settings
// returns an error.
func TestRunPull_ProjectFlagNotInSettings(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPullTestIO(t)

	// --project references a project that does not exist in settings.
	exit := RunPull([]string{"--project", "nonexistent", "my-issues"})
	if exit != 1 {
		t.Fatalf("RunPull(refresh --project nonexistent) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "nonexistent") {
		t.Fatalf("stderr = %q, want 'nonexistent' in error", stderr.String())
	}
}

// TestRunPull_CwdNotMatching verifies that passing --cwd
// with a directory that does not match any project's mirror_dir
// returns an error.
func TestRunPull_CwdNotMatching(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withPullTestIO(t)

	// --cwd points to a directory that does not match the project's mirror_dir.
	exit := RunPull([]string{"--cwd", filepath.Join(tmpDir, "outside"), "my-issues"})
	if exit != 1 {
		t.Fatalf("RunPull(refresh --cwd outside) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestRunMirrorArchiveSweep_ProjectFlagNotInSettings verifies that
// passing --project with a project name that does not exist in
// settings returns an error.
// TestRunMirrorArchiveSweep_Apply_ServiceCreationFailure verifies that
// --apply returns an error when the archive service cannot be created.
// TestRunMirrorArchiveSweep_CwdNotMatching verifies that passing --cwd
// with a directory that does not match any project's mirror_dir
// returns an error.
// TestSaveMirrorYAML_WriteError verifies that saveMirrorYAML returns an
// error when the file cannot be written.
func TestSaveMirrorYAML_WriteError(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a read-only directory to force a write error.
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(roDir, 0o555); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	roPath := filepath.Join(roDir, "mirror.yml")
	m := &mirror.Mirror{}
	err := saveMirrorYAML(roPath, m)
	if err == nil {
		t.Fatal("expected error writing to read-only directory")
	}
	if !strings.Contains(err.Error(), "write mirror file") {
		t.Fatalf("error = %q, want 'write mirror file'", err.Error())
	}
}

// TestResolveMirrorContext_Success verifies that resolveMirrorContext
// returns a valid context when the cwd matches a project's mirror_dir.
func TestResolveMirrorContext_Success(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	// Use the project's actual mirror_dir as cwd.
	mirrorDir := filepath.Join(tmpDir, "mirror")
	ctx, ok := resolveMirrorContext(settings, "", mirrorDir, "refresh")
	if !ok {
		t.Fatal("expected resolved context")
	}
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.Key != "TEST" {
		t.Fatalf("ctx.Key = %q, want 'TEST'", ctx.Key)
	}
}

func TestRunPull_IncludesComments(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("my-issues", []*schema.Issue{
		{Identity: schema.IssueIdentity{Key: "TEST-1", Type: "bug"}},
	})
	fake.SetIssue("TEST-1", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "TEST-1", Type: "bug"},
		Summary:  "Has comments",
	})
	fake.SetComments("TEST-1", []schema.Comment{
		{Author: "carol", Created: "2024-02-01T00:00:00Z", Body: "mirrored comment"},
	})
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withPullTestIO(t)
	if exit := RunPull([]string{"--project", "test", "my-issues"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(tmpDir, "local", "TEST-1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "### carol") || !strings.Contains(string(data), "mirrored comment") {
		t.Errorf("TEST-1.md lacks comments:\n%s", data)
	}

	fake.SetIssue("TEST-1", &schema.Issue{Identity: schema.IssueIdentity{Key: "TEST-1", Type: "bug"}, Summary: "Has comments"})
	fake.SetErr("comments", errors.New("nope"))
	if exit := RunPull([]string{"--full", "--project", "test", "my-issues"}); exit != 0 {
		t.Fatalf("exit = %d on comments failure", exit)
	}
	if !strings.Contains(stderr.String(), "warning: cannot fetch comments for TEST-1") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunPull_PerIssueFailureIsolated(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`)
	t.Setenv("HOME", homeDir)

	fake := jira.NewFakeTransport()
	var list []*schema.Issue
	for _, k := range []string{"TEST-1", "TEST-2", "TEST-3"} {
		iss := &schema.Issue{
			Identity:     schema.IssueIdentity{Key: schema.IssueKey(k), Type: "bug", Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}},
			MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
			Summary:      "Summary " + k,
		}
		fake.SetIssue(k, iss)
		list = append(list, iss)
	}
	fake.SetIssuesByScope("my-issues", list)
	// A directory squatting on TEST-2.md makes only that issue fail to write.
	if err := os.MkdirAll(filepath.Join(tmpDir, "local", "TEST-2.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withPullTestIO(t)

	exit := RunPull([]string{"--project", "test", "my-issues"})
	t.Logf("stdout: %q", stdout.String())
	t.Logf("stderr: %q", stderr.String())
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	for _, k := range []string{"TEST-1", "TEST-3"} {
		if _, err := os.ReadFile(filepath.Join(tmpDir, "local", k+".md")); err != nil {
			t.Errorf("%s.md not written: %v", k, err)
		}
	}
	for _, want := range []string{"TEST-2: ", "pulled 2, failed 1", "jirafs pull TEST-2"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	data, err := os.ReadFile(filepath.Join(tmpDir, "mirror", "mirror.yml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "key: TEST-1") || !strings.Contains(got, "key: TEST-3") || strings.Contains(got, "key: TEST-2") {
		t.Errorf("mirror.yml = %q, want members TEST-1 and TEST-3 only", got)
	}
}
