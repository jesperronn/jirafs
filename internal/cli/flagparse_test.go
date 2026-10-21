package cli

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
)

func newTestFS() (*flag.FlagSet, *bool, *string) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	b := fs.Bool("b", false, "")
	s := fs.String("project", "", "")
	return fs, b, s
}

func TestParseInterspersed(t *testing.T) {
	cases := []struct {
		name string
		args []string
		pos  []string
		b    bool
		s    string
	}{
		{"bool after positional", []string{"A", "-b"}, []string{"A"}, true, ""},
		{"value after positional space", []string{"A", "--project", "X"}, []string{"A"}, false, "X"},
		{"value after positional equals", []string{"A", "--project=X", "B"}, []string{"A", "B"}, false, "X"},
		{"flags before", []string{"-b", "--project", "X", "A"}, []string{"A"}, true, "X"},
		{"double dash passthrough", []string{"A", "--", "-b", "--project"}, []string{"A", "-b", "--project"}, false, ""},
		{"none", nil, nil, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fs, b, s := newTestFS()
			pos, err := parseInterspersed(fs, c.args)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(pos, c.pos) || *b != c.b || *s != c.s {
				t.Fatalf("pos=%v b=%v s=%q", pos, *b, *s)
			}
			if len(c.pos) > 0 && !reflect.DeepEqual(fs.Args(), c.pos) {
				t.Fatalf("fs.Args()=%v", fs.Args())
			}
		})
	}
}

func TestParseInterspersedUnknownFlag(t *testing.T) {
	fs, _, _ := newTestFS()
	if _, err := parseInterspersed(fs, []string{"A", "--nope"}); err == nil {
		t.Fatal("want error for unknown flag")
	}
}

func TestExportIssueFlagAfterKey(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	fake := exportFakeIssue("PROJ-42")
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return fake, nil })
	stdout, stderr := withExportTestIO(t)
	if exit := RunExport([]string{"issue", "PROJ-42", "--stdout", "--project", "test"}); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	if !strings.Contains(stdout.String(), "PROJ-42") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestSyncDryRunKeyFirst(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	local := "---\nkey: PROJ-42\ntype: story\nproject:\n  type: project\n  value: PROJ\n---\n\nSummary: Local\n"
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-42.md"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := jira.NewFakeTransport()
	client := &writeCountingClient{Client: fake}
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return client, nil })
	_, stderr := withSyncTestIO(t)
	if exit := RunSync([]string{"PROJ-42", "--dry-run", "--project", "test"}); strings.Contains(stderr.String(), "positional") || strings.Contains(stderr.String(), "flag provided") {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	if client.writes != 0 {
		t.Fatalf("writes=%d", client.writes)
	}
}

func TestPushFileFirstDryRun(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	out, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	if code := RunPush([]string{p, "--cwd", dir, "--dry-run"}); code != 0 {
		t.Fatalf("exit=%d err=%s", code, errBuf)
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Fatalf("stdout=%q", out)
	}
}

func TestNewFlagsAroundTemplate(t *testing.T) {
	for _, args := range [][]string{
		{"--summary", "Order story", "--dry-run", "story"},
		{"story", "--summary", "Order story", "--dry-run"},
		{"--summary", "Order story", "story", "--dry-run"},
	} {
		localDir := setupNewTest(t)
		out, errBuf := withNewTestIO(t)
		code := RunNew(append(append([]string{}, args...), "--cwd", localDir))
		if code != 0 || !strings.Contains(out.String(), "would create") {
			t.Fatalf("args=%v code=%d out=%q err=%q", args, code, out, errBuf)
		}
	}
}
