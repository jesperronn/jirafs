package cli

import (
	stdcontext "context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

// searchRecorder serves a fixed result set for every search and records the
// JQL it was asked for; every other call goes to the embedded fake.
type searchRecorder struct {
	jira.Client
	issues []*schema.Issue
	jqls   []string
}

func (s *searchRecorder) SearchIssues(_ stdcontext.Context, jql string) ([]*schema.Issue, int, error) {
	s.jqls = append(s.jqls, jql)
	return s.issues, len(s.issues), nil
}

type incrEnv struct {
	tmpDir string
	rec    *searchRecorder
	stdout *strings.Builder
	stderr *strings.Builder
}

func (e *incrEnv) path(parts ...string) string {
	return filepath.Join(append([]string{e.tmpDir}, parts...)...)
}

func (e *incrEnv) mirrorYAML(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(e.path("mirror", "mirror.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func (e *incrEnv) refresh(t *testing.T, args ...string) int {
	t.Helper()
	e.stdout.Reset()
	e.stderr.Reset()
	return RunPull(append([]string{"--project", "test"}, append(args, "my-issues")...))
}

func newIncrEnv(t *testing.T, scopeExtra string) *incrEnv {
	t.Helper()
	tmpDir := t.TempDir()
	homeDir := writeSettings(t, tmpDir)
	writeMirrorWithScopes(t, tmpDir, `
scopes:
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`+scopeExtra)
	t.Setenv("HOME", homeDir)

	fake := jira.NewFakeTransport()
	rec := &searchRecorder{Client: fake}
	for _, k := range []string{"TEST-1", "TEST-2", "TEST-3"} {
		iss := &schema.Issue{
			Identity:     schema.IssueIdentity{Key: schema.IssueKey(k), Type: "bug", Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}},
			MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
			Summary:      "Summary " + k,
			Context:      schema.IssueContext{Updated: "2026-01-01T09:00:00.000+0000"},
		}
		fake.SetIssue(k, iss)
		rec.issues = append(rec.issues, iss)
	}
	withPullClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return rec, nil
	})
	oldNow := pullNow
	pullNow = func() time.Time { return time.Date(2026, 1, 2, 10, 30, 0, 0, time.UTC) }
	t.Cleanup(func() { pullNow = oldNow })

	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	oldOut, oldErr := pullStdout, pullStderr
	pullStdout, pullStderr = stdout, stderr
	t.Cleanup(func() { pullStdout, pullStderr = oldOut, oldErr })
	return &incrEnv{tmpDir: tmpDir, rec: rec, stdout: stdout, stderr: stderr}
}

func wantSince(t *testing.T) string {
	t.Helper()
	return time.Date(2026, 1, 2, 10, 29, 0, 0, time.UTC).Local().Format("2006/01/02 15:04")
}

func TestRunPull_IncrementalCursor(t *testing.T) {
	e := newIncrEnv(t, "")

	if exit := e.refresh(t); exit != 0 {
		t.Fatalf("first refresh exit = %d, stderr = %q", exit, e.stderr.String())
	}
	t.Logf("first refresh output:\n%s", e.stdout.String())
	if !strings.Contains(e.stdout.String(), "fetched 3, updated 3, unchanged 0") {
		t.Fatalf("first stdout = %q", e.stdout.String())
	}
	if !strings.Contains(e.mirrorYAML(t), "last_synced: \"2026-01-02T10:30:00Z\"") && !strings.Contains(e.mirrorYAML(t), "last_synced: 2026-01-02T10:30:00Z") {
		t.Fatalf("mirror.yml lacks last_synced:\n%s", e.mirrorYAML(t))
	}
	first := e.rec.jqls[0]
	if strings.Contains(first, "updated >=") {
		t.Fatalf("first JQL = %q, want no cursor clause", first)
	}

	// Pin mtimes in the past so a rewrite would be visible.
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	before := map[string][]byte{}
	for _, k := range []string{"TEST-1", "TEST-2", "TEST-3"} {
		p := e.path("local", k+".md")
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		before[k] = data
	}

	if exit := e.refresh(t); exit != 0 {
		t.Fatalf("second refresh exit = %d, stderr = %q", exit, e.stderr.String())
	}
	t.Logf("second refresh output:\n%s", e.stdout.String())
	if !strings.Contains(e.stdout.String(), "fetched 3, updated 0, unchanged 3") {
		t.Fatalf("second stdout = %q", e.stdout.String())
	}
	second := e.rec.jqls[len(e.rec.jqls)-1]
	want := `(assignee = currentUser()) AND updated >= "` + wantSince(t) + `"`
	if second != want {
		t.Fatalf("second JQL = %q, want %q", second, want)
	}
	t.Logf("JQL first refresh:  %s", first)
	t.Logf("JQL second refresh: %s", second)
	for k, data := range before {
		p := e.path("local", k+".md")
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(data) {
			t.Errorf("%s bytes changed by refresh", k)
		}
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if !st.ModTime().Equal(old) {
			t.Errorf("%s mtime = %v, want untouched %v", k, st.ModTime(), old)
		}
	}
	if !strings.Contains(e.mirrorYAML(t), "remote_updated:") {
		t.Errorf("remote_updated lost after refresh:\n%s", e.mirrorYAML(t))
	}
}

func TestRunPull_FullIgnoresCursor(t *testing.T) {
	e := newIncrEnv(t, "    last_synced: \"2026-01-02T08:00:00Z\"\n")
	if exit := e.refresh(t, "--full"); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, e.stderr.String())
	}
	for _, q := range e.rec.jqls {
		if q != "assignee = currentUser()" {
			t.Errorf("JQL = %q, want the plain scope JQL with --full", q)
		}
	}
	if !strings.Contains(e.mirrorYAML(t), "2026-01-02T10:30:00Z") {
		t.Errorf("--full should still advance the cursor:\n%s", e.mirrorYAML(t))
	}
}

func TestRunPull_DryRunKeepsCursor(t *testing.T) {
	e := newIncrEnv(t, "    last_synced: \"2026-01-02T08:00:00Z\"\n")
	if exit := e.refresh(t, "--dry-run"); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, e.stderr.String())
	}
	if !strings.Contains(e.rec.jqls[0], "updated >=") {
		t.Errorf("dry-run JQL = %q, want cursor clause", e.rec.jqls[0])
	}
	if !strings.Contains(e.mirrorYAML(t), "2026-01-02T08:00:00Z") || strings.Contains(e.mirrorYAML(t), "10:30:00Z") {
		t.Errorf("dry-run changed the cursor:\n%s", e.mirrorYAML(t))
	}
}

func TestRunPull_FailureKeepsCursor(t *testing.T) {
	e := newIncrEnv(t, "    last_synced: \"2026-01-02T08:00:00Z\"\n")
	if err := os.MkdirAll(e.path("local", "TEST-2.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if exit := e.refresh(t); exit != 1 {
		t.Fatalf("exit = %d, want 1; stderr = %q", exit, e.stderr.String())
	}
	got := e.mirrorYAML(t)
	if !strings.Contains(got, "2026-01-02T08:00:00Z") || strings.Contains(got, "10:30:00Z") {
		t.Errorf("failed refresh advanced the cursor:\n%s", got)
	}
}

func TestRunPull_KeepsLocalEdits(t *testing.T) {
	e := newIncrEnv(t, "")
	if exit := e.refresh(t); exit != 0 {
		t.Fatalf("first refresh exit = %d, stderr = %q", exit, e.stderr.String())
	}
	p := e.path("local", "TEST-1.md")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), "Summary TEST-1", "Locally edited", 1)
	if edited == string(data) {
		t.Fatalf("summary not found in %s", data)
	}
	if err := os.WriteFile(p, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	// Jira moved on for TEST-1: the remote updated timestamp differs.
	e.rec.issues[0].Context.Updated = "2026-01-02T09:59:00.000+0000"

	if exit := e.refresh(t, "--full"); exit != 0 {
		t.Fatalf("second refresh exit = %d, stderr = %q", exit, e.stderr.String())
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != edited {
		t.Errorf("locally edited file was overwritten:\n%s", got)
	}
	if !strings.Contains(e.stdout.String(), "kept local edits 1") {
		t.Errorf("stdout = %q, want kept local edits summary", e.stdout.String())
	}
	if !strings.Contains(e.stderr.String(), "TEST-1: local edits kept") || !strings.Contains(e.stderr.String(), "jirafs status") {
		t.Errorf("stderr = %q, want local edits hint", e.stderr.String())
	}
	if !strings.Contains(e.mirrorYAML(t), "2026-01-02T09:59:00.000+0000") {
		t.Errorf("remote_updated not recorded for kept issue:\n%s", e.mirrorYAML(t))
	}
}

func TestRunPull_BuiltinScopeStoresCursor(t *testing.T) {
	e := newIncrEnv(t, "")
	writeMirrorWithScopes(t, e.tmpDir, "")
	if exit := e.refresh(t); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, e.stderr.String())
	}
	got := e.mirrorYAML(t)
	if !strings.Contains(got, "name: my-issues") || !strings.Contains(got, "last_synced") {
		t.Errorf("built-in scope cursor not stored:\n%s", got)
	}
}
