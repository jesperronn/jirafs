package cli

import (
	"bytes"
	stdcontext "context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/archive"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

// writeCountingClient wraps a client and counts remote write calls.
type writeCountingClient struct {
	jira.Client
	writes int
}

func (c *writeCountingClient) UpdateIssue(ctx stdcontext.Context, key string, issue *schema.Issue) (*schema.Issue, error) {
	c.writes++
	return c.Client.UpdateIssue(ctx, key, issue)
}

func (c *writeCountingClient) CreateIssue(ctx stdcontext.Context, issue *schema.Issue) (string, error) {
	c.writes++
	return c.Client.CreateIssue(ctx, issue)
}

// snapshotTree returns path -> content (dirs map to "<dir>") for everything
// under root, so tests can assert nothing changed.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			snap[p] = "<dir>"
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		snap[p] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snap
}

func assertTreeUnchanged(t *testing.T, root string, before map[string]string) {
	t.Helper()
	if after := snapshotTree(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("files under %s changed during dry run\nbefore: %v\nafter:  %v", root, keysOf(before), keysOf(after))
	}
}

func keysOf(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestDryRunConflicts(t *testing.T) {
	var buf bytes.Buffer
	if dryRunConflicts(&buf, "jirafs x", true, false) || dryRunConflicts(&buf, "jirafs x", false, true) || buf.Len() != 0 {
		t.Fatal("single flag must not conflict")
	}
	if !dryRunConflicts(&buf, "jirafs x", true, true) || !strings.Contains(buf.String(), "jirafs x: --dry-run and --apply cannot be used together") {
		t.Fatalf("buf = %q", buf.String())
	}
	if writeVerb(filepath.Join(t.TempDir(), "nope")) != "create" || writeVerb(t.TempDir()) != "overwrite" {
		t.Fatal("writeVerb")
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	local := "---\nkey: PROJ-42\ntype: story\nproject:\n  type: project\n  value: PROJ\n---\n\nSummary: Local modified summary\n"
	if err := os.WriteFile(filepath.Join(localDir, "PROJ-42.md"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42", Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"}},
		Summary:  "Remote original summary",
	})
	client := &writeCountingClient{Client: fake}
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return client, nil })
	stdout, stderr := withSyncTestIO(t)
	before := snapshotTree(t, tmpDir)

	if exit := RunSync([]string{"--project", "test", "--dry-run", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if client.writes != 0 {
		t.Fatalf("remote writes = %d, want 0", client.writes)
	}
	assertTreeUnchanged(t, tmpDir, before)
	for _, want := range []string{"operation(s) for issue PROJ-42", "dry run: nothing was written; to run it for real: jirafs sync PROJ-42"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "applied") {
		t.Errorf("stdout claims applied:\n%s", stdout.String())
	}
}

func TestSyncDryRunWithApplyIsError(t *testing.T) {
	_, stderr := withSyncTestIO(t)
	if exit := RunSync([]string{"--dry-run", "--apply", "PROJ-1"}); exit != 2 {
		t.Fatalf("exit = %d, want 2", exit)
	}
	if !strings.Contains(stderr.String(), "--dry-run and --apply cannot be used together") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestMirrorRefreshDryRunWritesNothing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	writeMirrorWithScopes(t, tmpDir, "\nscopes:\n  - name: my-issues\n    type: jql\n    target: assignee = currentUser()\n")
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIssue(t, localDir, "TEST-1.md", "existing\n")
	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("my-issues", []*schema.Issue{
		{Identity: schema.IssueIdentity{Key: "TEST-2", Type: "story"}},
		{Identity: schema.IssueIdentity{Key: "TEST-1", Type: "bug"}},
	})
	withMirrorClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return fake, nil })
	stdout, stderr := withMirrorTestIO(t)
	before := snapshotTree(t, tmpDir)

	if exit := RunMirror([]string{"refresh", "--project", "test", "--dry-run", "my-issues"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	assertTreeUnchanged(t, tmpDir, before)
	for _, want := range []string{
		"would write 2 issue file(s)",
		"would overwrite " + filepath.Join(localDir, "TEST-1.md"),
		"would create " + filepath.Join(localDir, "TEST-2.md"),
		"to run it for real: jirafs mirror refresh my-issues",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestMirrorRefreshDryRunErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	writeMirrorWithScopes(t, tmpDir, "\nscopes:\n  - name: my-issues\n    type: jql\n    target: x\n")
	fake := jira.NewFakeTransport()
	fake.SetErr("search", errors.New("boom"))
	withMirrorClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return fake, nil })
	_, stderr := withMirrorTestIO(t)
	if exit := RunMirror([]string{"refresh", "--project", "test", "--dry-run", "my-issues"}); exit != 1 || !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}

	// A project without local_dirs is an error.
	settings := t.TempDir()
	t.Setenv("HOME", settings)
	if err := os.MkdirAll(filepath.Join(settings, ".jirafs"), 0o755); err != nil {
		t.Fatal(err)
	}
	toml := "version = 1\n[instances.default]\nbase_url = \"https://x.atlassian.net\"\nauth_type = \"basic\"\n[projects.test]\nkey = \"TEST\"\ninstance = \"default\"\nmirror_dir = \"" + tmpDir + "/mirror\"\n"
	if err := os.WriteFile(filepath.Join(settings, ".jirafs", "settings.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr = withMirrorTestIO(t)
	if exit := RunMirror([]string{"refresh", "--project", "test", "--dry-run", "my-issues"}); exit != 1 || !strings.Contains(stderr.String(), "no local_dirs") {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
}

type failingArchiveService struct{ calls int }

func (f *failingArchiveService) Archive(string, string, string, string) error {
	f.calls++
	return errors.New("must not be called")
}

func TestArchiveSweepDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	writeMirror(t, tmpDir)
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIssue(t, localDir, "TEST-1.md", "---\nkey: TEST-1\ntype: story\nproject:\n  type: project\n  value: TEST\nremote_metadata:\n  remote_version: \"1\"\n  content_hash: \"abc\"\n  resolved_status: \"resolved\"\n---\n\nSummary: First issue\n")
	svc := &failingArchiveService{}
	old := archiveServiceFactory
	archiveServiceFactory = func(*config.Settings, *context.Context, string) (archive.Service, error) { return svc, nil }
	t.Cleanup(func() { archiveServiceFactory = old })
	stdout, stderr := withMirrorTestIO(t)
	before := snapshotTree(t, tmpDir)

	if exit := RunMirror([]string{"archive-sweep", "--project", "test", "--dry-run"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	assertTreeUnchanged(t, tmpDir, before)
	if svc.calls != 0 {
		t.Fatalf("archive service called %d times", svc.calls)
	}
	for _, want := range []string{"1 eligible issue(s)", "TEST-1", "to run it for real: jirafs mirror archive-sweep --apply"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout.String())
		}
	}

	if exit := RunMirror([]string{"archive-sweep", "--project", "test", "--dry-run", "--apply"}); exit != 2 {
		t.Fatalf("dry-run+apply exit = %d, want 2", exit)
	}
	if !strings.Contains(stderr.String(), "--dry-run and --apply cannot be used together") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	assertTreeUnchanged(t, tmpDir, before)
}

func TestExportIssueDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	fake := exportFakeIssue("PROJ-7")
	withExportClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return fake, nil })
	stdout, stderr := withExportTestIO(t)
	localDir := filepath.Join(tmpDir, "local")
	dest := filepath.Join(localDir, "PROJ-7.md")

	// Target missing: would create, and the directory is not created.
	before := snapshotTree(t, tmpDir)
	if exit := RunExport([]string{"issue", "--project", "test", "--dry-run", "PROJ-7"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	assertTreeUnchanged(t, tmpDir, before)
	if !strings.Contains(stdout.String(), "would create "+dest) || !strings.Contains(stdout.String(), "to run it for real: jirafs export issue PROJ-7") {
		t.Fatalf("stdout = %q", stdout.String())
	}

	// Target exists: would overwrite, content untouched.
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIssue(t, localDir, "PROJ-7.md", "precious\n")
	stdout.Reset()
	before = snapshotTree(t, tmpDir)
	if exit := RunExport([]string{"issue", "--project", "test", "--dry-run", "PROJ-7"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	assertTreeUnchanged(t, tmpDir, before)
	if !strings.Contains(stdout.String(), "would overwrite "+dest) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestPushDryRunFlag(t *testing.T) {
	dir := setupNewTest(t)
	fake := jira.NewFakeTransport()
	out, errBuf := withPushTestIO(t, fake)
	p := writeDraft(t, dir)
	client := &writeCountingClient{Client: fake}
	oldFactory := pushClientFactory
	pushClientFactory = func(*config.Settings, *context.Context, string) (jira.Client, error) { return client, nil }
	t.Cleanup(func() { pushClientFactory = oldFactory })
	before := snapshotTree(t, dir)

	if code := RunPush([]string{p, "--dry-run", "--cwd", dir}); code != 0 {
		t.Fatalf("exit=%d err=%s", code, errBuf)
	}
	assertTreeUnchanged(t, dir, before)
	if client.writes != 0 {
		t.Fatalf("remote writes = %d", client.writes)
	}
	if !strings.Contains(out.String(), "dry run: nothing was written; to run it for real: jirafs push "+p+" --apply") {
		t.Fatalf("stdout = %q", out.String())
	}

	if code := RunPush([]string{p, "--dry-run", "--apply", "--cwd", dir}); code != 2 {
		t.Fatalf("dry-run+apply exit = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "--dry-run and --apply cannot be used together") {
		t.Fatalf("stderr = %q", errBuf)
	}
	assertTreeUnchanged(t, dir, before)
	if client.writes != 0 {
		t.Fatalf("remote writes = %d", client.writes)
	}
}

func TestArchiveIssueDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	writeMirror(t, tmpDir)
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeArchiveIssue(t, localDir, "ABC-1", "Archived test issue")
	chdir(t, localDir)
	stdout, stderr := withArchiveTestIO(t)
	before := snapshotTree(t, tmpDir)

	if code := RunArchive([]string{"issue", "ABC-1", "--project", "test", "--dry-run"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	assertTreeUnchanged(t, tmpDir, before)
	for _, want := range []string{"would archive", filepath.Join("_archive", "ABC-1.md"), "to run it for real: jirafs archive issue ABC-1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestNewDryRun(t *testing.T) {
	localDir := setupNewTest(t)
	out, errBuf := withNewTestIO(t)
	before := snapshotTree(t, localDir)
	args := []string{"story", "--summary", "Dry story", "--dry-run", "--cwd", localDir}
	if code := RunNew(args); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errBuf)
	}
	assertTreeUnchanged(t, localDir, before)
	if !strings.Contains(out.String(), "would create "+filepath.Join(localDir, "dry-story.md")) {
		t.Fatalf("stdout = %q", out.String())
	}

	if err := os.WriteFile(filepath.Join(localDir, "dry-story.md"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	before = snapshotTree(t, localDir)
	if code := RunNew(args); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errBuf)
	}
	assertTreeUnchanged(t, localDir, before)
	if !strings.Contains(out.String(), "would refuse") || !strings.Contains(out.String(), "--force") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestArchiveIssueDryRunReportsIneligible(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	writeMirror(t, tmpDir)
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeArchiveIssue(t, localDir, "ABC-1", "Pinned issue")
	p := filepath.Join(localDir, "ABC-1.md")
	data, _ := os.ReadFile(p)
	if err := os.WriteFile(p, []byte(strings.Replace(string(data), "resolved_status: resolved\n", "resolved_status: resolved\npinned: true\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, localDir)
	stdout, stderr := withArchiveTestIO(t)
	before := snapshotTree(t, tmpDir)
	if code := RunArchive([]string{"issue", "ABC-1", "--project", "test", "--dry-run"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	assertTreeUnchanged(t, tmpDir, before)
	for _, want := range []string{"would skip", "not archive-eligible"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout.String(), "would archive") {
		t.Errorf("must not claim it would archive:\n%s", stdout)
	}
}

// setupSyncWriteCount prepares a changed local issue and a write-counting
// fake client; it returns the client, the temp root and the local file path.
func setupSyncWriteCount(t *testing.T) (*writeCountingClient, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(localDir, "PROJ-42.md")
	local := "---\nkey: PROJ-42\ntype: story\nproject:\n  type: project\n  value: PROJ\n---\n\nSummary: Local modified summary\n"
	if err := os.WriteFile(localPath, []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := jira.NewFakeTransport()
	fake.SetIssue("PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42", Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"}},
		Summary:  "Remote original summary",
	})
	client := &writeCountingClient{Client: fake}
	withSyncClientFactory(t, func(*config.Settings, *context.Context, string) (jira.Client, error) { return client, nil })
	return client, tmpDir, localPath
}

func TestSyncWithoutApplyIsDryRun(t *testing.T) {
	client, tmpDir, _ := setupSyncWriteCount(t)
	stdout, stderr := withSyncTestIO(t)
	before := snapshotTree(t, tmpDir)

	if exit := RunSync([]string{"PROJ-42", "--project", "test"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if client.writes != 0 {
		t.Fatalf("remote writes = %d, want 0", client.writes)
	}
	assertTreeUnchanged(t, tmpDir, before)
	want := "to run it for real: jirafs sync PROJ-42 --apply --project test"
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
	}
	if strings.Contains(stdout.String(), "applied") {
		t.Fatalf("stdout claims applied:\n%s", stdout.String())
	}
}

func TestSyncApplyWithDryRunIsUsageError(t *testing.T) {
	client, tmpDir, _ := setupSyncWriteCount(t)
	_, stderr := withSyncTestIO(t)
	before := snapshotTree(t, tmpDir)

	if exit := RunSync([]string{"--project", "test", "--apply", "--dry-run", "PROJ-42"}); exit != 2 {
		t.Fatalf("exit = %d, want 2", exit)
	}
	if !strings.Contains(stderr.String(), "jirafs sync: --dry-run and --apply cannot be used together") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if client.writes != 0 {
		t.Fatalf("remote writes = %d", client.writes)
	}
	assertTreeUnchanged(t, tmpDir, before)
}

func TestSyncWriteLocalHasNoEffectInDryRun(t *testing.T) {
	client, tmpDir, _ := setupSyncWriteCount(t)
	stdout, stderr := withSyncTestIO(t)
	before := snapshotTree(t, tmpDir)

	if exit := RunSync([]string{"--project", "test", "--write-local", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if client.writes != 0 {
		t.Fatalf("remote writes = %d, want 0", client.writes)
	}
	assertTreeUnchanged(t, tmpDir, before)
	want := "jirafs sync PROJ-42 --apply --write-local --project test"
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
	}
}

func TestSyncApplyWithoutWriteLocalLeavesLocalFile(t *testing.T) {
	client, _, localPath := setupSyncWriteCount(t)
	_, stderr := withSyncTestIO(t)
	before, _ := os.ReadFile(localPath)

	if exit := RunSync([]string{"--project", "test", "--apply", "PROJ-42"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr.String())
	}
	if client.writes == 0 {
		t.Fatal("--apply performed no Jira writes")
	}
	after, _ := os.ReadFile(localPath)
	if string(before) != string(after) {
		t.Fatalf("local file changed without --write-local:\n%s", after)
	}
}
