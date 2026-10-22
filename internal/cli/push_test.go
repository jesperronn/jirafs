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
	"github.com/jirafs/jirafs/internal/schema"
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

// addCreateDefaults appends create_defaults to the test project's settings.
func addCreateDefaults(t *testing.T, defaults string) {
	t.Helper()
	p := filepath.Join(os.Getenv("HOME"), ".jirafs", "settings.toml")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(data), "\n[state]", "\n[projects.test.create_defaults]\n"+defaults+"\n[state]", 1)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPushPayloadMergesCreateDefaults(t *testing.T) {
	dir := setupNewTest(t)
	addCreateDefaults(t, "customfield_12601 = { value = \"Default Team\" }\ncustomfield_20000 = \"dflt\"\n")
	fake := jira.NewFakeTransport()
	out, errBuf := withPushTestIO(t, fake)
	p := filepath.Join(dir, "d.md")
	draft := strings.Replace(filledStory, "\n---\n", "\ncustom_fields:\n  customfield_12601:\n    value: Own Team\n---\n", 1)
	if err := os.WriteFile(p, []byte(draft), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := RunPush([]string{p, "--cwd", dir}); code != 0 {
		t.Fatalf("exit=%d err=%s", code, errBuf)
	}
	s := out.String()
	for _, want := range []string{`"customfield_12601": {`, `"value": "Own Team"`, `"customfield_20000": "dflt"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "Default Team") {
		t.Errorf("draft value must win:\n%s", s)
	}
}

func TestPushInvalidCreateDefaults(t *testing.T) {
	dir := setupNewTest(t)
	addCreateDefaults(t, "bogus = 1\n")
	out, errBuf := withPushTestIO(t, jira.NewFakeTransport())
	_ = out
	p := writeDraft(t, dir)
	if code := RunPush([]string{p, "--cwd", dir}); code != 1 || !strings.Contains(errBuf.String(), "invalid create_defaults") {
		t.Fatalf("got %s", errBuf)
	}
}

func teamMeta(n int) []jira.CreateMetaField {
	var av []jira.AllowedValue
	for i := 1; i <= n; i++ {
		av = append(av, jira.AllowedValue{ID: string(rune('0' + i%10)), Value: "Team " + string(rune('A'+i-1))})
	}
	return []jira.CreateMetaField{
		{ID: "summary", Name: "Summary", Required: true},
		{ID: "reporter", Name: "Reporter", Required: true},
		{ID: "customfield_12601", Name: "Team", Required: true, AllowedValues: av},
		{ID: "customfield_7", Name: "Optional", Required: false},
		{ID: "customfield_8", Name: "Defaulted", Required: true, HasDefault: true},
		{ID: "priority", Name: "Priority", Required: true},
	}
}

func TestPushDryRunListsMissingRequiredFields(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	fake.SetCreateMeta("TEST", "Story", teamMeta(12))
	out, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	if code := RunPush([]string{p, "--cwd", dir}); code != 0 {
		t.Fatalf("exit=%d err=%s", code, errBuf)
	}
	s := out.String()
	for _, want := range []string{
		"missing required field: Team (customfield_12601)",
		"allowed values: Team A [id 1]", "(2 more)",
		"custom_fields:", `customfield_12601: {value: "Team A"}`,
		"[projects.test.create_defaults]", `customfield_12601 = { value = "Team A" }`,
		"missing required field: Priority (priority)", "standard field",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	for _, bad := range []string{"Reporter", "Optional", "Defaulted", "(summary)"} {
		if strings.Contains(s, bad) {
			t.Errorf("must not report %q", bad)
		}
	}
}

func TestPushApplyRefusesMissingRequired(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	fake.SetCreateMeta("TEST", "Story", teamMeta(1))
	_, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 1 || !strings.Contains(errBuf.String(), "customfield_12601") || !strings.Contains(errBuf.String(), "refusing to create") {
		t.Fatalf("err=%s", errBuf)
	}
	if _, err := os.Stat(p); err != nil {
		t.Error("draft must remain")
	}
	if _, err := fake.FetchIssue(nil, "TEST-1"); err == nil {
		t.Error("must not create")
	}
}

func TestPushRequiredSatisfiedByDefaultsAndDraft(t *testing.T) {
	dir := setupNewTest(t)
	addCreateDefaults(t, "customfield_12601 = { value = \"Team A\" }\n")
	fake := jira.NewFakeTransport()
	meta := teamMeta(1)[:3]
	fake.SetCreateMeta("TEST", "Story", meta)
	_, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 0 {
		t.Fatalf("exit=%d err=%s", code, errBuf)
	}
}

func TestPushCreateMetaFailureWarns(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	fake.SetErr("createmeta", errors.New("boom"))
	out, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 0 || !strings.Contains(errBuf.String(), "warning: cannot check required fields: boom") || !strings.Contains(out.String(), "created TEST-1") {
		t.Fatalf("err=%s out=%s", errBuf, out)
	}
}

func TestPushDryRunNoClientWarns(t *testing.T) {
	dir := setupNewTest(t)
	_, errBuf := withPushTestIO(t, jira.NewFakeTransport())
	pushClientFactory = func(*config.Settings, *context.Context, string) (jira.Client, error) {
		return nil, errors.New("no creds")
	}
	p := writeDraft(t, dir)
	if code := RunPush([]string{p, "--cwd", dir}); code != 0 || !strings.Contains(errBuf.String(), "warning: cannot check required fields (no Jira client: no creds)") {
		t.Fatalf("err=%s", errBuf)
	}
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 1 || !strings.Contains(errBuf.String(), "cannot create Jira client") {
		t.Fatalf("apply err=%s", errBuf)
	}
}

func TestPushApplyHintPreservesPathAndFlags(t *testing.T) {
	if got, want := pushApplyHint("drafts/my draft.md", "test", "/w"), "jirafs push 'drafts/my draft.md' --apply --project test --cwd /w"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got, want := pushApplyHint("d.md", "", ""), "jirafs push d.md --apply"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	issue := &schema.Issue{}
	if h := syncHint(issue); strings.Contains(h, "jirafs sync") {
		t.Errorf("keyless hint must not suggest sync: %q", h)
	}
}
