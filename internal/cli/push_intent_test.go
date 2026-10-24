package cli

import (
	stdcontext "context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jirafs/jirafs/internal/jira"
)

// intentFiles lists the intent records under the test project's mirror dir.
func intentFiles(t *testing.T, _ string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".jirafs", "settings.toml"))
	if err != nil {
		t.Fatal(err)
	}
	mirror := regexp.MustCompile(`mirror_dir = "([^"]+)"`).FindStringSubmatch(string(data))[1]
	m, _ := filepath.Glob(filepath.Join(mirror, ".jirafs", "intents", "*.yml"))
	return m
}

func TestPushLostResponseRetryAdoptsSameKey(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	out, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)

	fake.SetErr("create-lost", errors.New("timeout"))
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 1 {
		t.Fatalf("first push exit=%d", code)
	}
	if len(intentFiles(t, dir)) != 1 {
		t.Fatalf("intent must survive a failed create: %v", intentFiles(t, dir))
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("draft must survive")
	}
	fake.SetErr("", nil)
	out.Reset()
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 0 {
		t.Fatalf("retry exit=%d err=%s", code, errBuf)
	}
	if got := fake.Created(); got != 1 {
		t.Fatalf("created=%d want 1", got)
	}
	if !strings.Contains(out.String(), "found TEST-1 created by an earlier attempt; not creating a duplicate") {
		t.Errorf("output:\n%s", out)
	}
	t.Logf("retry output:\n%s", out)
	if _, err := os.Stat(filepath.Join(dir, "TEST-1.md")); err != nil {
		t.Error("KEY.md missing")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("draft should be removed")
	}
	if len(intentFiles(t, dir)) != 0 {
		t.Error("intent should be removed")
	}
	issue, _ := fake.FetchIssue(stdcontext.Background(), "TEST-1")
	for _, l := range issue.Labels {
		if strings.HasPrefix(l, intentLabelPrefix) {
			t.Errorf("label %s should be removed", l)
		}
	}
}

func TestPushRetryAfterFailedCreateCreatesOnce(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	out, _ := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	fake.SetErr("create", errors.New("boom"))
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 1 {
		t.Fatal("expected failure")
	}
	in, _ := loadIntent(intentFiles(t, dir)[0])
	fake.SetErr("", nil)
	out.Reset()
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 0 {
		t.Fatal("retry failed")
	}
	if fake.Created() != 1 {
		t.Errorf("created=%d", fake.Created())
	}
	if strings.Contains(out.String(), "earlier attempt") {
		t.Errorf("must not claim adoption:\n%s", out)
	}
	if got := fake.Searches(); len(got) != 1 || got[0] != in.JQL() {
		t.Errorf("searches=%v", got)
	}
}

func TestPushIntentLifecycleAndSearchFailure(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	_, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	fake.SetErr("create", errors.New("boom"))
	RunPush([]string{p, "--apply", "--cwd", dir})
	fake.SetErr("search", errors.New("down"))
	errBuf.Reset()
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 1 || !strings.Contains(errBuf.String(), "earlier attempt") {
		t.Fatalf("search failure should fail: %s", errBuf)
	}
	if fake.Created() != 0 {
		t.Error("must not create when the check failed")
	}
	// Damaged record is replaced.
	fake.SetErr("", nil)
	ip := intentFiles(t, dir)[0]
	_ = os.WriteFile(ip, []byte(":: not yaml ["), 0o644)
	if code := RunPush([]string{p, "--apply", "--cwd", dir}); code != 0 {
		t.Fatalf("exit with damaged intent: %s", errBuf)
	}
	if len(intentFiles(t, dir)) != 0 {
		t.Error("intent should be gone")
	}
}

func TestStaleIntents(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if got := staleIntents(dir, now); got != nil {
		t.Errorf("no dir: %v", got)
	}
	_ = saveIntent(filepath.Join(dir, ".jirafs", "intents", "old.yml"), createIntent{Token: "aa", Timestamp: now.Add(-8 * 24 * time.Hour)})
	_ = saveIntent(filepath.Join(dir, ".jirafs", "intents", "new.yml"), createIntent{Token: "bb", Timestamp: now.Add(-time.Hour)})
	_ = os.WriteFile(filepath.Join(dir, ".jirafs", "intents", "junk.yml"), []byte("x: ["), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".jirafs", "intents", "note.txt"), nil, 0o644)
	_ = os.Mkdir(filepath.Join(dir, ".jirafs", "intents", "sub.yml"), 0o755)
	got := staleIntents(dir, now)
	if len(got) != 1 || !strings.Contains(got[0], "old.yml: 8 days old, label jirafs-aa") {
		t.Errorf("got %v", got)
	}
}

func TestNewIntentTokenUnique(t *testing.T) {
	a, _ := newIntentToken()
	b, _ := newIntentToken()
	if len(a) != 16 || a == b {
		t.Errorf("%q %q", a, b)
	}
}
