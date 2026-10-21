package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jirafs/jirafs/internal/codec"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
)

func withPushTestIO(t *testing.T, fake *jira.FakeTransport) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	oo, oe, of, on := pushStdout, pushStderr, pushClientFactory, pushNow
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	pushStdout, pushStderr = out, errBuf
	pushClientFactory = func(*config.Settings, *context.Context, string) (jira.Client, error) { return fake, nil }
	pushNow = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	t.Cleanup(func() { pushStdout, pushStderr, pushClientFactory, pushNow = oo, oe, of, on })
	return out, errBuf
}

func writeDraft(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "add-dark-mode.md")
	if err := os.WriteFile(p, []byte(filledStory), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPushDryRunDefault(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	out, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	if code := RunPush([]string{p, "--cwd", dir}); code != 0 {
		t.Fatalf("exit=%d err=%s", code, errBuf)
	}
	for _, want := range []string{"would create Story in project TEST: Add dark mode", `"key": "TEST"`, "jirafs push " + p + " --apply"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("draft should be untouched: %v", err)
	}
	if _, err := fake.FetchIssue(nil, "TEST-1"); err == nil {
		t.Error("dry run must not create")
	}
}

func TestPushApplyRewritesAndRenames(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	out, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 0 {
		t.Fatalf("exit=%d err=%s", code, errBuf)
	}
	newPath := filepath.Join(dir, "TEST-1.md")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("old draft should be removed")
	}
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	issue, pe := codec.ParseIssue(string(data))
	if pe != nil {
		t.Fatal(pe)
	}
	if issue.Identity.Key != "TEST-1" || issue.RemoteMetadata.StateFile == "draft" || issue.Summary != "Add dark mode" {
		t.Errorf("issue=%+v", issue)
	}
	for _, want := range []string{"created TEST-1", newPath, "https://jira.example.com/browse/TEST-1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	// A second push of the rewritten file is refused.
	errBuf.Reset()
	if code := RunPush([]string{newPath, "--apply", "--cwd", dir}); code != 1 || !strings.Contains(errBuf.String(), "not a draft") || !strings.Contains(errBuf.String(), "jirafs sync TEST-1") {
		t.Fatalf("want refusal with hint, got %s", errBuf)
	}
}

func TestPushErrors(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	_, errBuf := withPushTestIO(t, fake)
	if code := RunPush(nil); code != 1 || !strings.Contains(errBuf.String(), "jirafs push <draft.md>") {
		t.Errorf("no args: %s", errBuf)
	}
	if code := RunPush([]string{filepath.Join(dir, "none.md")}); code != 1 {
		t.Error("missing file should fail")
	}
	bad := filepath.Join(dir, "bad.md")
	_ = os.WriteFile(bad, []byte("---\ntype: [\n---\n"), 0o644)
	if code := RunPush([]string{bad}); code != 1 {
		t.Error("bad yaml should fail")
	}
	nosum := filepath.Join(dir, "nosum.md")
	_ = os.WriteFile(nosum, []byte("---\ntype: story\nstate: draft\n---\n"), 0o644)
	errBuf.Reset()
	if code := RunPush([]string{nosum, "--cwd", dir}); code != 1 || !strings.Contains(errBuf.String(), `"summary"`) {
		t.Errorf("no summary: %s", errBuf)
	}
	p := writeDraft(t, dir)
	fake.SetErr("create", errors.New("boom"))
	errBuf.Reset()
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 1 || !strings.Contains(errBuf.String(), "boom") {
		t.Errorf("create err: %s", errBuf)
	}
	if _, err := os.Stat(p); err != nil {
		t.Error("draft must survive a failed create")
	}
	if code := RunPush([]string{"--help"}); code != 0 {
		t.Error("help should succeed")
	}
	if code := RunPush([]string{"--bogus"}); code != 1 {
		t.Error("bad flag should fail")
	}
}

func TestPushFlagFirstAndProjectOverride(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	out, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	if code := RunPush([]string{"--cwd", dir, "--project", "TEST", p}); code != 0 {
		t.Fatalf("exit=%d err=%s", code, errBuf)
	}
	if !strings.Contains(out.String(), "project TEST") {
		t.Errorf("out=%s", out)
	}
}
