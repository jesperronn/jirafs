package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

// goneMovedEnv pulls TEST-1..3, then makes TEST-2 gone (with a local edit) and
// TEST-3 moved to TEST-12 in Jira.
func goneMovedEnv(t *testing.T) *incrEnv {
	t.Helper()
	e := newIncrEnv(t, "")
	if exit := e.refresh(t); exit != 0 {
		t.Fatalf("first pull exit = %d, stderr = %q", exit, e.stderr.String())
	}
	p := e.path("local", "TEST-2.md")
	edited := strings.Replace(readFile(t, p), "Summary TEST-2", "Edited locally", 1)
	if err := os.WriteFile(p, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := e.rec.Client.(*jira.FakeTransport)
	moved := *e.rec.issues[2]
	moved.Identity.Key = "TEST-12"
	fake.SetIssue("TEST-3", &moved)
	fake.SetIssue("TEST-12", &moved)
	fake.SetIssue("TEST-2", nil)
	e.rec.issues = e.rec.issues[:1]
	return e
}

func TestRunPull_FullGoneAndMoved(t *testing.T) {
	e := goneMovedEnv(t)

	// Dry run prints the same lines with "would" and changes nothing.
	if exit := e.refresh(t, "--full", "--dry-run"); exit != 0 {
		t.Fatalf("dry-run exit = %d, stderr = %q", exit, e.stderr.String())
	}
	out := e.stdout.String()
	for _, want := range []string{
		"would gone TEST-2: move to _archive/TEST-2.md with your local edits kept (restore: mv _archive/TEST-2.md .)",
		"would moved TEST-3 -> TEST-12: jirafs pull TEST-12",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run stdout missing %q:\n%s", want, out)
		}
	}
	if !exists(e.path("local", "TEST-2.md")) || !exists(e.path("local", "TEST-3.md")) || exists(e.path("local", "_archive")) || exists(e.path("local", "TEST-12.md")) {
		t.Fatal("dry run changed files")
	}

	if exit := e.refresh(t, "--full"); exit != 0 {
		t.Fatalf("pull exit = %d, stderr = %q", exit, e.stderr.String())
	}
	out = e.stdout.String()
	t.Logf("pull output:\n%s", out)
	for _, want := range []string{
		"gone TEST-2: moved to _archive/TEST-2.md with your local edits kept (restore: mv _archive/TEST-2.md .)",
		"moved TEST-3 -> TEST-12: jirafs pull TEST-12",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
	gone := readFile(t, e.path("local", "_archive", "TEST-2.md"))
	if !strings.Contains(gone, "remote_state: 'gone'") || !strings.Contains(gone, "Edited locally") {
		t.Fatalf("archived gone file wrong:\n%s", gone)
	}
	old := readFile(t, e.path("local", "_archive", "TEST-3.md"))
	if !strings.Contains(old, "remote_state: 'moved'") || !strings.Contains(old, "moved_to: 'TEST-12'") {
		t.Fatalf("archived moved file wrong:\n%s", old)
	}
	if !strings.Contains(readFile(t, e.path("local", "TEST-12.md")), "TEST-12") {
		t.Fatal("new key file not written")
	}
	if exists(e.path("local", "TEST-2.md")) || exists(e.path("local", "TEST-3.md")) {
		t.Fatal("old live files remain")
	}
	if exists(mirror.BasePath(e.path("mirror"), "TEST-3")) || !exists(mirror.BasePath(e.path("mirror"), "TEST-12")) {
		t.Fatal("base snapshot not moved to new key")
	}
	y := e.mirrorYAML(t)
	if strings.Contains(y, "TEST-2") || strings.Contains(y, "TEST-3\n") || !strings.Contains(y, "TEST-12") {
		t.Fatalf("mirror membership wrong:\n%s", y)
	}

	// Archived files are not pushable and say why.
	_, perr := withPushTestIO(t, jira.NewFakeTransport())
	if exit := RunPush([]string{"--project", "test", e.path("local", "_archive", "TEST-2.md")}); exit == 0 {
		t.Fatal("push of archived gone file must fail")
	}
	if !strings.Contains(perr.String(), "is archived") || !strings.Contains(perr.String(), "no longer has") {
		t.Fatalf("push stderr = %q", perr.String())
	}
}

func TestRunPullIssue_GoneAndMoved(t *testing.T) {
	e := goneMovedEnv(t)
	if exit := RunPull([]string{"--project", "test", "TEST-2", "--dry-run"}); exit != 0 {
		t.Fatalf("dry-run exit = %d, stderr = %q", exit, e.stderr.String())
	}
	if !strings.Contains(e.stdout.String(), "would gone TEST-2") || !exists(e.path("local", "TEST-2.md")) {
		t.Fatalf("dry run wrong: %q", e.stdout.String())
	}
	e.stdout.Reset()
	if exit := RunPull([]string{"--project", "test", "TEST-2"}); exit != 0 {
		t.Fatalf("gone exit = %d, stderr = %q", exit, e.stderr.String())
	}
	if !strings.Contains(e.stdout.String(), "gone TEST-2: moved to _archive/TEST-2.md with your local edits kept") || !exists(e.path("local", "_archive", "TEST-2.md")) {
		t.Fatalf("gone wrong: %q", e.stdout.String())
	}
	e.stdout.Reset()
	if exit := RunPull([]string{"--project", "test", "TEST-3"}); exit != 0 {
		t.Fatalf("moved exit = %d, stderr = %q", exit, e.stderr.String())
	}
	if !strings.Contains(e.stdout.String(), "moved TEST-3 -> TEST-12: jirafs pull TEST-12") || !exists(e.path("local", "TEST-12.md")) || !exists(e.path("local", "_archive", "TEST-3.md")) {
		t.Fatalf("moved wrong: %q", e.stdout.String())
	}
	// A missing issue without a local file keeps the plain error.
	e.stderr.Reset()
	if exit := RunPull([]string{"--project", "test", "TEST-99"}); exit != 1 {
		t.Fatalf("unknown issue exit = %d", exit)
	}
}

func TestRunPull_MovedKeepsLocalEditsAtNewKey(t *testing.T) {
	e := goneMovedEnv(t)
	newPath := e.path("local", "TEST-12.md")
	doc := schema.StampSyncedHash(strings.Replace(readFile(t, e.path("local", "TEST-3.md")), "TEST-3", "TEST-12", -1))
	if err := os.WriteFile(newPath, []byte(strings.Replace(doc, "Summary TEST-12", "Mine", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if exit := RunPull([]string{"--project", "test", "TEST-3"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, e.stderr.String())
	}
	if !strings.Contains(readFile(t, newPath), "Mine") {
		t.Fatal("local edits at new key were overwritten")
	}
}

func TestArchiveRemoteIssue_Errors(t *testing.T) {
	dir := t.TempDir()
	if _, err := archiveRemoteIssue(dir+"/TEST-1.md", "", "TEST-1", "", false); err == nil {
		t.Fatal("want read error")
	}
	bad := dir + "/TEST-2.md"
	if err := os.WriteFile(bad, []byte("not an issue"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := archiveRemoteIssue(bad, "", "TEST-2", "", false); err == nil {
		t.Fatal("want parse error")
	}
	if err := os.MkdirAll(dir+"/_archive", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/_archive/TEST-2.md", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := archiveRemoteIssue(bad, "", "TEST-2", "", false); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want already exists", err)
	}
	if !exists(bad) {
		t.Fatal("source must stay when the archive slot is taken")
	}
}

func TestArchivedPushHint(t *testing.T) {
	if h := archivedPushHint(schema.RemoteMetadata{RemoteState: "gone"}); !strings.Contains(h, "no longer has") {
		t.Fatal(h)
	}
	if h := archivedPushHint(schema.RemoteMetadata{RemoteState: "moved", MovedTo: "X-2"}); !strings.Contains(h, "jirafs pull X-2") {
		t.Fatal(h)
	}
	if archivedPushHint(schema.RemoteMetadata{}) != "" {
		t.Fatal("want empty hint")
	}
}
