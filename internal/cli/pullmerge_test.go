package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

// editLocal replaces old by new in the TEST-1 file and returns its previous
// and new content.
func (e *incrEnv) editLocal(t *testing.T, old, repl string) (before, after string) {
	t.Helper()
	p := e.path("local", "TEST-1.md")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	after = strings.Replace(string(data), old, repl, 1)
	if after == string(data) {
		t.Fatalf("%q not found in %s", old, data)
	}
	if err := os.WriteFile(p, []byte(after), 0o644); err != nil {
		t.Fatal(err)
	}
	return string(data), after
}

func (e *incrEnv) localFile(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(e.path("local", "TEST-1.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func (e *incrEnv) baseIssue(t *testing.T) *schema.Issue {
	t.Helper()
	b, ok := mirror.LoadBase(e.path("mirror"), "TEST-1")
	if !ok {
		t.Fatal("no base snapshot for TEST-1")
	}
	return b
}

func (e *incrEnv) backups(t *testing.T) []string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(mirror.BackupDir(e.path("mirror")), "TEST-1-*.md"))
	return matches
}

// remoteMoves changes TEST-1 on the fake Jira.
func (e *incrEnv) remoteMoves(summary string, labels ...string) {
	i := e.rec.issues[0]
	if summary != "" {
		i.Summary = summary
	}
	if labels != nil {
		i.Labels = labels
	}
	i.Context.Updated = "2026-01-02T09:59:00.000+0000"
}

func firstPull(t *testing.T) *incrEnv {
	t.Helper()
	e := newIncrEnv(t, "")
	if exit := e.refresh(t); exit != 0 {
		t.Fatalf("first pull exit = %d, stderr = %q", exit, e.stderr.String())
	}
	return e
}

func TestPullMerge_AutoMergesDisjointFields(t *testing.T) {
	e := firstPull(t)
	before, edited := e.editLocal(t, "Summary TEST-1", "Locally edited")
	e.remoteMoves("", "remote-label")

	if exit := e.refresh(t, "--full"); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, e.stderr.String())
	}
	out := e.stdout.String()
	if !strings.Contains(out, "\nmerged TEST-1 ") || !strings.Contains(out, "merged 1") {
		t.Errorf("stdout = %q, want a merged TEST-1 line", out)
	}
	got := e.localFile(t)
	if !strings.Contains(got, "summary: 'Locally edited'") || !strings.Contains(got, "remote-label") {
		t.Errorf("merged file lacks both changes:\n%s", got)
	}
	if strings.Contains(got, "conflicts:") {
		t.Errorf("clean merge recorded conflicts:\n%s", got)
	}
	if classifyLocalIssue(e.path("local", "TEST-1.md"), "") != localEdited {
		t.Error("merged file should still read as locally modified")
	}
	b := e.backups(t)
	if len(b) != 1 {
		t.Fatalf("backups = %v", b)
	}
	if data, _ := os.ReadFile(b[0]); string(data) != edited {
		t.Errorf("backup differs from the pre-merge local file")
	}
	base := e.baseIssue(t)
	if base.Summary != "Summary TEST-1" || len(base.Labels) != 1 || base.Labels[0] != "remote-label" {
		t.Errorf("base = summary %q labels %v, want old summary and remote label", base.Summary, base.Labels)
	}
	t.Logf("pull output:\n%s\nmerged file:\n%s\nbackup unchanged vs pre-merge (%d bytes; before pull %d bytes)", out, got, len(edited), len(before))

	// A second pull with the remote unchanged leaves the merged file alone.
	if exit := e.refresh(t, "--full"); exit != 0 {
		t.Fatalf("second exit = %d", exit)
	}
	if e.localFile(t) != got || len(e.backups(t)) != 1 {
		t.Error("second pull modified the merged file or saved another backup")
	}
}

func TestPullMerge_ConflictKeepsLocalAndRecordsRemote(t *testing.T) {
	e := firstPull(t)
	_, edited := e.editLocal(t, "Summary TEST-1", "Local title")
	e.remoteMoves("Remote title")

	if exit := e.refresh(t, "--full"); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, e.stderr.String())
	}
	out := e.stdout.String()
	if !strings.Contains(out, "conflict TEST-1 summary (backup: ") || !strings.Contains(out, "review: jirafs diff TEST-1)") || !strings.Contains(out, "conflicted 1") {
		t.Errorf("stdout = %q", out)
	}
	got := e.localFile(t)
	for _, want := range []string{"summary: 'Local title'", "conflicts:", "field: summary", "remote: Remote title", "base: Summary TEST-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("file lacks %q:\n%s", want, got)
		}
	}
	if e.baseIssue(t).Summary != "Summary TEST-1" {
		t.Errorf("base summary advanced to %q", e.baseIssue(t).Summary)
	}
	if b := e.backups(t); len(b) != 1 {
		t.Fatalf("backups = %v", b)
	} else if data, _ := os.ReadFile(b[0]); string(data) != edited {
		t.Error("backup differs from the pre-merge local file")
	}
	t.Logf("pull output:\n%s\nconflicted file:\n%s", out, got)

	// The remote moves again: the conflict is reproduced against the same base.
	e.remoteMoves("Remote title 2")
	e.rec.issues[0].Context.Updated = "2026-01-02T09:59:30.000+0000"
	if exit := e.refresh(t, "--full"); exit != 0 {
		t.Fatalf("second exit = %d", exit)
	}
	again := e.localFile(t)
	if !strings.Contains(again, "remote: Remote title 2") || !strings.Contains(again, "summary: 'Local title'") || strings.Count(again, "conflicts:") != 1 {
		t.Errorf("conflict not reproduced:\n%s", again)
	}
	if len(e.backups(t)) != 2 {
		t.Errorf("backups = %v, want one per merge", e.backups(t))
	}
}

func TestPullMerge_NoBaseNeverOverwrites(t *testing.T) {
	e := firstPull(t)
	e.editLocal(t, "Summary TEST-1", "Local title")
	if err := os.Remove(mirror.BasePath(e.path("mirror"), "TEST-1")); err != nil {
		t.Fatal(err)
	}
	e.remoteMoves("Remote title")

	if exit := e.refresh(t, "--full"); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, e.stderr.String())
	}
	if !strings.Contains(e.stdout.String(), "conflict TEST-1 summary (no base snapshot)") {
		t.Errorf("stdout = %q", e.stdout.String())
	}
	got := e.localFile(t)
	if !strings.Contains(got, "summary: 'Local title'") || !strings.Contains(got, "remote: Remote title") {
		t.Errorf("file:\n%s", got)
	}
	if _, ok := mirror.LoadBase(e.path("mirror"), "TEST-1"); ok {
		t.Error("a base was recorded although the file is still conflicted")
	}
	if len(e.backups(t)) != 1 {
		t.Errorf("backups = %v", e.backups(t))
	}
}

func TestPullMerge_LocalEditAlreadyOnRemoteIsClean(t *testing.T) {
	e := firstPull(t)
	e.editLocal(t, "Summary TEST-1", "Same everywhere")
	e.remoteMoves("Same everywhere")

	if exit := e.refresh(t, "--full"); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, e.stderr.String())
	}
	if !strings.Contains(e.stdout.String(), "merged TEST-1") {
		t.Errorf("stdout = %q", e.stdout.String())
	}
	if classifyLocalIssue(e.path("local", "TEST-1.md"), "") == localEdited {
		t.Error("file identical to the remote should be synced, not modified")
	}
	if e.baseIssue(t).Summary != "Same everywhere" {
		t.Error("base not advanced")
	}
}

func TestPullMerge_SingleIssueMerges(t *testing.T) {
	e := firstPull(t)
	e.editLocal(t, "Summary TEST-1", "Local title")
	e.remoteMoves("", "remote-label")
	e.stdout.Reset()
	e.stderr.Reset()
	if exit := RunPull([]string{"--project", "test", "TEST-1"}); exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", exit, e.stderr.String())
	}
	if !strings.HasPrefix(e.stdout.String(), "merged TEST-1 ") {
		t.Errorf("stdout = %q", e.stdout.String())
	}
	if got := e.localFile(t); !strings.Contains(got, "summary: 'Local title'") || !strings.Contains(got, "remote-label") {
		t.Errorf("file:\n%s", got)
	}

	// Remote unchanged since the merge: kept, no new backup.
	e.stdout.Reset()
	if exit := RunPull([]string{"--project", "test", "TEST-1"}); exit != 0 {
		t.Fatalf("exit = %d", exit)
	}
	if !strings.HasPrefix(e.stdout.String(), "kept TEST-1 ") || len(e.backups(t)) != 1 {
		t.Errorf("stdout = %q backups = %v", e.stdout.String(), e.backups(t))
	}
}

func TestPullMerge_BackupFailureLeavesFileUntouched(t *testing.T) {
	e := firstPull(t)
	_, edited := e.editLocal(t, "Summary TEST-1", "Local title")
	e.remoteMoves("Remote title")
	if err := os.MkdirAll(e.path("mirror", ".jirafs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mirror.BackupDir(e.path("mirror")), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if exit := e.refresh(t, "--full"); exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if e.localFile(t) != edited {
		t.Error("file changed although the backup failed")
	}
	if !strings.Contains(e.stderr.String(), "TEST-1: merge into TEST-1.md: save backup") {
		t.Errorf("stderr = %q", e.stderr.String())
	}
}

func TestMergeEditedFile_Errors(t *testing.T) {
	dir := t.TempDir()
	remote := &schema.Issue{Identity: schema.IssueIdentity{Key: "TEST-9"}}
	if _, err := mergeEditedFile(filepath.Join(dir, "missing.md"), dir, remote, pullNow()); err == nil {
		t.Error("missing file: want error")
	}
	bad := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(bad, []byte("---\nkey: [unclosed\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := mergeEditedFile(bad, dir, remote, pullNow())
	if err != nil || rep.Kind != mergeConflicted || rep.Fields[0] != "file" {
		t.Fatalf("rep = %+v err = %v", rep, err)
	}
	if got, _ := os.ReadFile(bad); !strings.Contains(string(got), "unclosed") {
		t.Error("unparseable file was modified")
	}
	if len(mustGlob(t, filepath.Join(mirror.BackupDir(dir), "TEST-9-*.md"))) != 1 {
		t.Error("no backup of the unparseable file")
	}
	if _, err := mergeEditedFile(bad, "", remote, pullNow()); err == nil {
		t.Error("empty mirror dir: want backup error")
	}
}

func mustGlob(t *testing.T, pattern string) []string {
	t.Helper()
	m, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
