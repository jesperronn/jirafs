package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

// TestRunMirror_NoSubcommand verifies that calling RunMirror with no
// subcommand returns exit code 1 and prints a usage hint.
func TestRunMirror_NoSubcommand(t *testing.T) {
	exit := RunMirror([]string{})
	if exit != 1 {
		t.Errorf("RunMirror([]) = %d, want 1", exit)
	}
}

// TestRunMirror_UnknownSubcommand verifies that an unknown subcommand
// returns exit code 1.
func TestRunMirror_UnknownSubcommand(t *testing.T) {
	exit := RunMirror([]string{"bogus"})
	if exit != 1 {
		t.Errorf("RunMirror([\"bogus\"]) = %d, want 1", exit)
	}
}

// TestRunMirror_Help verifies that "help" returns exit code 0.
func TestRunMirror_Help(t *testing.T) {
	// Help runs before config.Load, so it should succeed regardless of
	// whether a settings file exists.
	exit := RunMirror([]string{"help"})
	if exit != 0 {
		t.Errorf("RunMirror([\"help\"]) = %d, want 0", exit)
	}
}

func withMirrorTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := mirrorStdout
	oldStderr := mirrorStderr
	mirrorStdout = stdout
	mirrorStderr = stderr
	t.Cleanup(func() {
		mirrorStdout = oldStdout
		mirrorStderr = oldStderr
	})
	return stdout, stderr
}

func withMirrorClientFactory(t *testing.T, factory func(*config.Settings, *context.Context, string) (jira.Client, error)) {
	t.Helper()
	oldFactory := mirrorClientFactory
	mirrorClientFactory = factory
	t.Cleanup(func() {
		mirrorClientFactory = oldFactory
	})
}

// TestRunMirrorArchiveSweep_NoProject verifies that archive-sweep
// returns error when no project can be resolved.
func TestRunMirrorArchiveSweep_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunMirror([]string{"archive-sweep"})
	if exit != 1 {
		t.Errorf("RunMirror([\"archive-sweep\"]) = %d, want 1", exit)
	}
}

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

func TestRunMirrorRefresh_MissingScope(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	_, stderr := withMirrorTestIO(t)
	exit := RunMirror([]string{"refresh"})
	if exit != 1 {
		t.Fatalf("RunMirror([\"refresh\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "missing scope name") {
		t.Fatalf("stderr = %q, want missing scope name", stderr.String())
	}
}

func TestRunMirrorRefresh_ResolvesProjectAndRefreshesScope(t *testing.T) {
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
	withMirrorClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withMirrorTestIO(t)

	exit := RunMirror([]string{"refresh", "--project", "test", "my-issues"})
	if exit != 0 {
		t.Fatalf("RunMirror(refresh) = %d, stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `added 2 issue(s) to scope "my-issues"`) {
		t.Fatalf("stdout = %q, want refresh summary", stdout.String())
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
}

func TestRunMirrorRefresh_ScopeNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	_, stderr := withMirrorTestIO(t)

	exit := RunMirror([]string{"refresh", "--project", "test", "my-issues"})
	if exit != 1 {
		t.Fatalf("RunMirror(refresh unknown scope) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), `scope "my-issues" not found`) {
		t.Fatalf("stderr = %q, want scope not found", stderr.String())
	}
}

func TestRunMirrorRefresh_TooManyArgs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	_, stderr := withMirrorTestIO(t)

	exit := RunMirror([]string{"refresh", "my-issues", "extra"})
	if exit != 1 {
		t.Fatalf("RunMirror(refresh too many args) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "too many positional arguments") {
		t.Fatalf("stderr = %q, want too many positional arguments", stderr.String())
	}
}

func TestRunMirrorRefresh_ClientCreationError(t *testing.T) {
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
	withMirrorClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, os.ErrPermission
	})
	_, stderr := withMirrorTestIO(t)

	exit := RunMirror([]string{"refresh", "--project", "test", "my-issues"})
	if exit != 1 {
		t.Fatalf("RunMirror(refresh client error) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create Jira client") {
		t.Fatalf("stderr = %q, want client creation error", stderr.String())
	}
}

func TestRunMirrorRefresh_SearchError(t *testing.T) {
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
	withMirrorClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withMirrorTestIO(t)

	exit := RunMirror([]string{"refresh", "--project", "test", "my-issues"})
	if exit != 1 {
		t.Fatalf("RunMirror(refresh search error) = %d, want 1", exit)
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

// TestRunMirrorArchiveSweep_EligibleIssues verifies that archive-sweep
// reports eligible issues without mutation.
func TestRunMirrorArchiveSweep_EligibleIssues(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Issue 1: resolved, out of scope → eligible.
	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: First issue
`)

	// Issue 2: open → not eligible.
	writeIssue(t, localDir, "TEST-2.md", `---
key: TEST-2
type: bug
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "def"
  resolved_status: "open"
---

Summary: Second issue
`)

	// Issue 3: resolved, out of scope → eligible.
	writeIssue(t, localDir, "TEST-3.md", `---
key: TEST-3
type: task
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "ghi"
  resolved_status: "resolved"
---

Summary: Third issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("RunMirror([\"archive-sweep\", \"--project\", \"test\"]) = %d, want 0", exit)
	}

	// Verify files were NOT modified (no mutation).
	for _, name := range []string{"TEST-1.md", "TEST-2.md", "TEST-3.md"} {
		data, err := os.ReadFile(filepath.Join(localDir, name))
		if err != nil {
			t.Errorf("ReadFile %s: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s was emptied by archive-sweep", name)
		}
	}
}

// TestRunMirrorArchiveSweep_NoEligibleIssues verifies that archive-sweep
// reports nothing when no issues are eligible.
func TestRunMirrorArchiveSweep_NoEligibleIssues(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "open"
---

Summary: Open issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

// TestRunMirrorArchiveSweep_PinnedIssue verifies that pinned issues
// are not reported as eligible.
func TestRunMirrorArchiveSweep_PinnedIssue(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
  pinned: true
---

Summary: Pinned issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

// TestRunMirrorArchiveSweep_UnsyncedIssue verifies that unsynced issues
// (zero remote metadata) are not reported as eligible.
func TestRunMirrorArchiveSweep_UnsyncedIssue(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
---

Summary: Unsynced issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

// TestRunMirrorArchiveSweep_ExplicitImportNotEligible verifies that
// explicitly imported issues are not reported as eligible.
func TestRunMirrorArchiveSweep_ExplicitImportNotEligible(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Mirror with an explicitly imported issue.
	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	mirrorYAML := `project:
  type: project
  value: TEST
issues:
  - key: TEST-1
    reason: manual
`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Issue 1 is explicitly imported + resolved → NOT eligible.
	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: Explicitly imported issue
`)

	// Issue 2 is out of scope + resolved → eligible.
	writeIssue(t, localDir, "TEST-2.md", `---
key: TEST-2
type: bug
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "def"
  resolved_status: "resolved"
---

Summary: Out-of-scope issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

// TestRunMirrorArchiveSweep_ScopeMemberNotEligible verifies that scope
// members are not reported as eligible.
func TestRunMirrorArchiveSweep_ScopeMemberNotEligible(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)

	// Mirror with a scope member.
	mirrorDir := filepath.Join(tmpDir, "mirror")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll mirror: %v", err)
	}
	mirrorYAML := `project:
  type: project
  value: TEST
scopes:
  - name: active
    type: jql
    target: status=Active
scope_members:
  - key: TEST-1
    scope: active
`
	if err := os.WriteFile(filepath.Join(mirrorDir, "mirror.yml"), []byte(mirrorYAML), 0o644); err != nil {
		t.Fatalf("WriteFile mirror: %v", err)
	}

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Issue 1 is a scope member + resolved → NOT eligible.
	writeIssue(t, localDir, "TEST-1.md", `---
key: TEST-1
type: story
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "abc"
  resolved_status: "resolved"
---

Summary: Scope member issue
`)

	// Issue 2 is out of scope + resolved → eligible.
	writeIssue(t, localDir, "TEST-2.md", `---
key: TEST-2
type: bug
project:
  type: project
  value: TEST
remote_metadata:
  remote_version: "1"
  content_hash: "def"
  resolved_status: "resolved"
---

Summary: Out-of-scope issue
`)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}

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
	_, stderr := withMirrorTestIO(t)

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
	if err := os.WriteFile(credsPath, []byte("api_token = \"token\"\n"), 0o644); err != nil {
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

	client, err := buildMirrorClient(settings, &context.Context{Instance: "default"}, ".")
	if err != nil {
		t.Fatalf("buildMirrorClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestLoadMirrorArchiveSweep_UnparseableIssue verifies that issues with
// invalid frontmatter are skipped without error.
func TestLoadMirrorArchiveSweep_UnparseableIssue(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirror(t, tmpDir)

	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll local: %v", err)
	}

	// Write a file with invalid frontmatter.
	if err := os.WriteFile(filepath.Join(localDir, "INVALID.md"), []byte(`---
key:
  invalid: yaml
---

Summary: Invalid issue
`), 0o644); err != nil {
		t.Fatalf("WriteFile INVALID.md: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	exit := RunMirror([]string{"archive-sweep", "--project", "test"})
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
}
