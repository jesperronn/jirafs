package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	icontext "github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

func testIssue(key string) *schema.Issue {
	return &schema.Issue{Identity: schema.IssueIdentity{Key: schema.IssueKey(key), Type: "Story"}}
}

func TestMaterializeScopeIssues_WritesFilesAndSkipsInvalid(t *testing.T) {
	localDir := filepath.Join(t.TempDir(), "nested", "local")
	settings := &config.Settings{Projects: map[string]config.Project{"p": {LocalDirs: []string{localDir}}}}
	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("sc", []*schema.Issue{testIssue("A-1"), nil, testIssue(""), testIssue("A-2")})
	out, _ := withMirrorTestIO(t)

	_, _, err := materializeScopeIssues(context.Background(), fake, settings, &icontext.Context{Name: "p"}, "sc")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	for _, k := range []string{"A-1", "A-2"} {
		data, rerr := os.ReadFile(filepath.Join(localDir, k+".md"))
		if rerr != nil {
			t.Fatalf("missing %s.md: %v", k, rerr)
		}
		if !strings.Contains(string(data), k) {
			t.Errorf("%s.md does not mention its key: %q", k, data)
		}
	}
	entries, _ := os.ReadDir(localDir)
	if len(entries) != 2 {
		t.Errorf("wrote %d files, want 2 (nil and keyless skipped)", len(entries))
	}
	if !strings.Contains(out.String(), "fetching 2/2") || !strings.Contains(out.String(), "fetched 2/2") {
		t.Errorf("progress output = %q, want fetching 2/2 and fetched 2/2", out.String())
	}
}

func TestMaterializeScopeIssues_EmptyScope(t *testing.T) {
	settings := &config.Settings{Projects: map[string]config.Project{"p": {LocalDirs: []string{t.TempDir()}}}}
	out, _ := withMirrorTestIO(t)
	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("empty", []*schema.Issue{})
	if _, _, err := materializeScopeIssues(context.Background(), fake, settings, &icontext.Context{Name: "p"}, "empty"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "fetched 0/0") {
		t.Errorf("output = %q", out.String())
	}
}

func TestMaterializeScopeIssues_Errors(t *testing.T) {
	fake := jira.NewFakeTransport()
	good := &config.Settings{Projects: map[string]config.Project{"p": {LocalDirs: []string{t.TempDir()}}}}
	fileAsDir := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(fileAsDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	failing := jira.NewFakeTransport()
	failing.SetErr("search", jira.NewHTTPErr(500, "down"))

	cases := []struct {
		name     string
		client   jira.Client
		settings *config.Settings
		proj     string
		want     string
	}{
		{"unknown project", fake, good, "zzz", "not found in settings"},
		{"no local dirs", fake, &config.Settings{Projects: map[string]config.Project{"p": {}}}, "p", "no local_dirs"},
		{"mkdir fails", fake, &config.Settings{Projects: map[string]config.Project{"p": {LocalDirs: []string{filepath.Join(fileAsDir, "sub")}}}}, "p", "create local directory"},
		{"search fails", failing, good, "p", "down"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withMirrorTestIO(t)
			_, _, err := materializeScopeIssues(context.Background(), c.client, c.settings, &icontext.Context{Name: c.proj}, "sc")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want containing %q", err, c.want)
			}
		})
	}
}

func TestRunMirrorRefresh_FlagAndArgErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	writeMirror(t, tmpDir)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"refresh", "--bogus"}, "invalid flags"},
		{[]string{"refresh", "--project", "test"}, "missing scope name"},
	} {
		_, stderr := withMirrorTestIO(t)
		if code := RunMirror(c.args); code != 1 || !strings.Contains(stderr.String(), c.want) {
			t.Errorf("RunMirror(%v) = %d stderr=%q", c.args, code, stderr.String())
		}
	}
}
