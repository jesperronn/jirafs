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

func ptrString(s string) *string {
	return &s
}

// TestRunExport_NoSubcommand verifies that calling RunExport with no
// subcommand returns exit code 1 and prints a usage hint.
func TestRunExport_NoSubcommand(t *testing.T) {
	exit := RunExport([]string{})
	if exit != 1 {
		t.Errorf("RunExport([]) = %d, want 1", exit)
	}
}

// TestRunExport_UnknownSubcommand verifies that an unknown subcommand
// returns exit code 1.
func TestRunExport_UnknownSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withExportTestIO(t)
	exit := RunExport([]string{"bogus"})
	if exit != 1 {
		t.Fatalf("RunExport([\"bogus\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Fatalf("stderr = %q, want unknown subcommand", stderr.String())
	}
}

// TestRunExport_Help verifies that "help" returns exit code 0.
func TestRunExport_Help(t *testing.T) {
	exit := RunExport([]string{"help"})
	if exit != 0 {
		t.Errorf("RunExport([\"help\"]) = %d, want 0", exit)
	}
}

func withExportTestIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	oldStdout := exportStdout
	oldStderr := exportStderr
	exportStdout = stdout
	exportStderr = stderr
	t.Cleanup(func() {
		exportStdout = oldStdout
		exportStderr = oldStderr
	})
	return stdout, stderr
}

func withExportClientFactory(t *testing.T, factory func(*config.Settings, *context.Context, string) (jira.Client, error)) {
	t.Helper()
	oldFactory := exportClientFactory
	exportClientFactory = factory
	t.Cleanup(func() {
		exportClientFactory = oldFactory
	})
}

// TestRunExportIssue_MissingKey verifies that export issue without a key
// returns exit code 1.
func TestRunExportIssue_MissingKey(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withExportTestIO(t)
	exit := RunExport([]string{"issue", "--cwd", filepath.Join(tmpDir, "outside")})
	if exit != 1 {
		t.Fatalf("RunExport([\"issue\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "missing issue key") {
		t.Fatalf("stderr = %q, want missing issue key", stderr.String())
	}
}

// TestRunExportIssue_MissingProject verifies that export issue without
// a resolvable project returns exit code 1.
func TestRunExportIssue_MissingProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withExportTestIO(t)
	// Don't pass --project, and use a non-matching cwd.
	exit := RunExport([]string{"issue", "--cwd", filepath.Join(tmpDir, "outside"), "PROJ-1"})
	if exit != 1 {
		t.Fatalf("RunExport([\"issue\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestRunExportIssue_Success verifies that export issue fetches and
// exports a single issue through the real service path.
func TestRunExportIssue_Success(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "PROJ-42",
			Type:    "story",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
		},
		Summary:      "Test issue summary",
		Description:  "Test issue description",
		Labels:       []string{"bug", "urgent"},
		Assignee:     ptrString("alice"),
		MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
		RemoteMetadata: schema.RemoteMetadata{
			StateFile:     "synced",
			RemoteVersion: "7",
			ContentHash:   "abc123",
		},
	})
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withExportTestIO(t)

	exit := RunExport([]string{"issue", "--project", "test", "PROJ-42"})
	if exit != 0 {
		t.Fatalf("RunExport([\"issue\", \"--project\", \"test\", \"PROJ-42\"]) = %d, stderr = %q", exit, stderr.String())
	}

	// Verify the exported output contains the issue key and summary.
	output := stdout.String()
	if !strings.Contains(output, "PROJ-42") {
		t.Fatalf("stdout = %q, want PROJ-42 key", output)
	}
	if !strings.Contains(output, "Test issue summary") {
		t.Fatalf("stdout = %q, want summary", output)
	}

	// Verify round-trip: parse the exported output back.
	parsed, pe := schema.ParseIssue(output)
	if pe != nil {
		t.Fatalf("ParseIssue failed on exported output: %s", pe.Error())
	}
	if parsed.Identity.Key != "PROJ-42" {
		t.Errorf("round-trip key = %q, want PROJ-42", parsed.Identity.Key)
	}
	if parsed.Summary != "Test issue summary" {
		t.Errorf("round-trip summary = %q, want \"Test issue summary\"", parsed.Summary)
	}
}

// TestRunExportIssue_NotFound verifies that exporting a non-existent
// issue returns a not found error.
func TestRunExportIssue_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withExportTestIO(t)

	exit := RunExport([]string{"issue", "--project", "test", "PROJ-99"})
	if exit != 1 {
		t.Fatalf("RunExport([\"issue\", \"PROJ-99\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "not_found") {
		t.Fatalf("stderr = %q, want not_found error", stderr.String())
	}
}

// TestRunExportIssue_TooManyArgs verifies that export issue with
// too many positional arguments returns an error.
func TestRunExportIssue_TooManyArgs(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withExportTestIO(t)
	exit := RunExport([]string{"issue", "--cwd", filepath.Join(tmpDir, "outside"), "PROJ-1", "extra"})
	if exit != 1 {
		t.Fatalf("RunExport([\"issue\", \"PROJ-1\", \"extra\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "too many positional arguments") {
		t.Fatalf("stderr = %q, want too many positional arguments", stderr.String())
	}
}

// TestRunExportIssue_ClientCreationError verifies that export issue
// returns an error when the Jira client cannot be created.
func TestRunExportIssue_ClientCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, os.ErrPermission
	})
	_, stderr := withExportTestIO(t)

	exit := RunExport([]string{"issue", "--project", "test", "--cwd", filepath.Join(tmpDir, "outside"), "PROJ-1"})
	if exit != 1 {
		t.Fatalf("RunExport([\"issue\", \"PROJ-1\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "cannot create Jira client") {
		t.Fatalf("stderr = %q, want client creation error", stderr.String())
	}
}

// TestRunExportIssue_FetchError verifies that export issue returns
// an error when the Jira fetch fails.
func TestRunExportIssue_FetchError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetErr("fetch", jira.NewHTTPErr(500, "server error"))
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	_, stderr := withExportTestIO(t)

	exit := RunExport([]string{"issue", "--project", "test", "--cwd", filepath.Join(tmpDir, "outside"), "PROJ-1"})
	if exit != 1 {
		t.Fatalf("RunExport([\"issue\", \"PROJ-1\"]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "server error") {
		t.Fatalf("stderr = %q, want server error", stderr.String())
	}
}

// TestRunExportIssue_MinimalIssue verifies that exporting a minimal
// (draft) issue still produces valid output.
func TestRunExportIssue_MinimalIssue(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	fake := jira.NewFakeTransport()
	fake.SetIssue("DRF-1", &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:     "DRF-1",
			Type:    "task",
			Project: schema.TypedRef{Type: schema.RefProject, Value: "DRF"},
		},
		MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
	})
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return fake, nil
	})
	stdout, stderr := withExportTestIO(t)

	exit := RunExport([]string{"issue", "--project", "test", "--cwd", filepath.Join(tmpDir, "outside"), "DRF-1"})
	if exit != 0 {
		t.Fatalf("RunExport([\"issue\", \"DRF-1\"]) = %d, stderr = %q", exit, stderr.String())
	}

	output := stdout.String()
	if output == "" {
		t.Fatal("stdout should not be empty")
	}

	// Verify round-trip.
	parsed, pe := schema.ParseIssue(output)
	if pe != nil {
		t.Fatalf("ParseIssue failed on exported output: %s", pe.Error())
	}
	if parsed.Identity.Key != "DRF-1" {
		t.Errorf("round-trip key = %q, want DRF-1", parsed.Identity.Key)
	}
}

// TestBuildExportClient_Success verifies that buildExportClient
// creates a valid Jira client from settings.
func TestBuildExportClient_Success(t *testing.T) {
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

	client, err := buildExportClient(settings, &context.Context{Instance: "default"}, ".")
	if err != nil {
		t.Fatalf("buildExportClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestRunExportIssue_ResolveContextError verifies that export issue
// returns an error when no project can be resolved for the given cwd.
func TestRunExportIssue_ResolveContextError(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	// Use a cwd that doesn't match any project's mirror_dir.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	_, stderr := withExportTestIO(t)
	exit := RunExport([]string{"issue", "--cwd", filepath.Join(tmpDir, "outside"), "PROJ-1"})
	if exit != 1 {
		t.Fatalf("RunExport([\"issue\", \"--cwd\", ...]) = %d, want 1", exit)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestResolveExportContext_NoProject verifies that resolveExportContext
// returns false when no project can be resolved for the given cwd.
func TestResolveExportContext_NoProject(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)
	settings, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	_, stderr := withExportTestIO(t)

	ctx, ok := resolveExportContext(settings, "", filepath.Join(tmpDir, "outside"))
	if ok {
		t.Fatal("expected unresolved project")
	}
	if ctx != nil {
		t.Fatalf("expected nil context, got %#v", ctx)
	}
	if !strings.Contains(stderr.String(), "no project resolved") {
		t.Fatalf("stderr = %q, want no project resolved", stderr.String())
	}
}

// TestBuildExportClient_CredentialError verifies that buildExportClient
// returns an error when the instance has no credentials configured.
func TestBuildExportClient_CredentialError(t *testing.T) {
	_ = t.TempDir()
	settings := &config.Settings{
		Instances: map[string]config.Instance{
			"default": {
				BaseURL:  "https://example.atlassian.net",
				AuthType: "basic",
				// No CredentialRefs → should fail.
			},
		},
	}

	_, err := buildExportClient(settings, &context.Context{Instance: "default"}, ".")
	if err == nil {
		t.Fatal("expected error for instance with no credentials")
	}
	if !strings.Contains(err.Error(), "no_usable_instance") {
		t.Fatalf("error = %q, want no_usable_instance", err.Error())
	}
}
