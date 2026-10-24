package cli

import (
	stdcontext "context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

// mixedPushFixture is changesFixture (clean TEST-1, modified TEST-2,
// conflicted TEST-4) plus one filled draft, dark-mode.md, and a fake Jira
// that knows the three existing issues.
func mixedPushFixture(t *testing.T) (string, *jira.FakeTransport) {
	t.Helper()
	local := changesFixture(t)
	if err := os.Remove(filepath.Join(local, "TEST-3.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "dark-mode.md"), []byte(filledStory), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := jira.NewFakeTransport()
	for _, key := range []string{"TEST-1", "TEST-2", "TEST-4"} {
		fake.SetIssue(key, &schema.Issue{
			Identity: schema.IssueIdentity{Key: schema.IssueKey(key), Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}},
			Summary:  "original",
		})
	}
	withPushTestIO(t, fake)
	return local, fake
}

func TestPushNoArgsMixedDryRunWritesNothing(t *testing.T) {
	local, fake := mixedPushFixture(t)
	out, errBuf := withPushTestIO(t, fake)
	before := map[string]string{}
	entries, _ := os.ReadDir(local)
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(local, e.Name()))
		before[e.Name()] = string(b)
	}

	if code := RunPush(nil); code != 0 {
		t.Fatalf("exit=%d err=%s out=%s", code, errBuf, out)
	}
	t.Logf("dry run output:\n%s", out)
	for _, want := range []string{
		"TEST-4  skipped (conflict)",
		"TEST-2  ok  would apply",
		"dark-mode.md  ok  would create Story",
		"2 ok, 1 skipped, 0 failed",
		"dry run: nothing was written; to run it for real: jirafs push --apply",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out.String(), "TEST-1  ") {
		t.Errorf("clean TEST-1 must not be listed:\n%s", out)
	}
	for name, content := range before {
		after, _ := os.ReadFile(filepath.Join(local, name))
		if string(after) != content {
			t.Errorf("%s changed during a dry run", name)
		}
	}
	if _, err := fake.FetchIssue(stdcontext.Background(), "TEST-5"); err == nil {
		t.Error("dry run must not create")
	}
	if got, _ := fake.FetchIssue(stdcontext.Background(), "TEST-2"); got.Summary != "original" {
		t.Errorf("dry run changed remote summary to %q", got.Summary)
	}
}

func TestPushNoArgsMixedApply(t *testing.T) {
	local, fake := mixedPushFixture(t)
	out, errBuf := withPushTestIO(t, fake)

	if code := RunPush([]string{"--apply"}); code != 0 {
		t.Fatalf("exit=%d err=%s out=%s", code, errBuf, out)
	}
	t.Logf("mixed --apply summary:\n%s", out)
	for _, want := range []string{
		"TEST-4  skipped (conflict)",
		"TEST-2  ok  applied",
		"ok  created from " + filepath.Join(local, "dark-mode.md"),
		"2 ok, 1 skipped, 0 failed",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if got, _ := fake.FetchIssue(stdcontext.Background(), "TEST-2"); got.Summary != "edited" {
		t.Errorf("remote TEST-2 summary = %q, want edited", got.Summary)
	}
	if got, _ := fake.FetchIssue(stdcontext.Background(), "TEST-4"); got.Summary != "original" {
		t.Errorf("conflicted TEST-4 was written: %q", got.Summary)
	}
	if _, err := os.Stat(filepath.Join(local, "dark-mode.md")); !os.IsNotExist(err) {
		t.Error("draft should be renamed after create")
	}
}

func TestPushMixedExitsOneWhenAnyFailed(t *testing.T) {
	_, fake := mixedPushFixture(t)
	out, errBuf := withPushTestIO(t, fake)
	fake.SetErr("update", jira.NewHTTPErr(500, "boom"))

	if code := RunPush([]string{"--apply"}); code != 1 {
		t.Fatalf("exit=%d, want 1; err=%s", code, errBuf)
	}
	for _, want := range []string{"TEST-2  failed  cannot update remote issue", "ok  created from", "TEST-4  skipped (conflict)", "1 ok, 1 skipped, 1 failed"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestPushExplicitConflictedKeySkipped(t *testing.T) {
	_, fake := mixedPushFixture(t)
	out, _ := withPushTestIO(t, fake)
	if code := RunPush([]string{"TEST-4", "--apply"}); code != 0 {
		t.Fatalf("a skipped conflict is not a failure; exit=%d", code)
	}
	if !strings.Contains(out.String(), "TEST-4  skipped (conflict)  remote changed since the last pull; review it: jirafs diff TEST-4") {
		t.Errorf("out=%s", out)
	}
}

func TestPushDryRunAndApplyExitsTwo(t *testing.T) {
	_, fake := mixedPushFixture(t)
	_, errBuf := withPushTestIO(t, fake)
	if code := RunPush([]string{"--dry-run", "--apply"}); code != 2 || !strings.Contains(errBuf.String(), "--dry-run and --apply cannot be used together") {
		t.Fatalf("exit=%d err=%s", code, errBuf)
	}
}

func TestPushTargetErrors(t *testing.T) {
	local, fake := mixedPushFixture(t)
	out, _ := withPushTestIO(t, fake)
	arch := filepath.Join(local, "arch.md")
	_ = os.WriteFile(arch, []byte("---\nkey: 'TEST-9'\ntype: 'story'\nproject: 'project:TEST'\nstate: 'archived'\nsummary: 'x'\n---\n"), 0o644)
	nokey := filepath.Join(local, "nokey.md")
	_ = os.WriteFile(nokey, []byte("---\ntype: 'story'\nproject: 'project:TEST'\nstate: 'synced'\nremote_version: '1'\nsummary: 'x'\n---\n"), 0o644)
	badkey := filepath.Join(local, "badkey.md")
	_ = os.WriteFile(badkey, []byte("---\nkey: 'TEST-8'\ntype: 'story'\nproject: 'project:TEST'\nstate: 'draft'\nsummary: 'x'\n---\n"), 0o644)
	garbage := filepath.Join(local, "garbage.md")
	_ = os.WriteFile(garbage, []byte("no frontmatter"), 0o644)

	code := RunPush([]string{arch, nokey, badkey, garbage, "--apply"})
	if code != 1 {
		t.Fatalf("exit=%d", code)
	}
	for _, want := range []string{"is archived", "has no issue key", "is a draft but already has issue key TEST-8", "invalid document", "0 ok, 0 skipped, 4 failed"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
