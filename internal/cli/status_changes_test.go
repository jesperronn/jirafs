package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

// changesFixture builds a workspace with one clean synced file (TEST-1), one
// edited file (TEST-2), one draft (TEST-3) and one edited file whose remote
// copy moved on (TEST-4), then points HOME and cwd at it. It returns the
// local directory.
func changesFixture(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	writeMirror(t, tmpDir)
	local := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	synced := func(key, updated string) string {
		doc := "---\nkey: '" + key + "'\ntype: 'story'\nproject: 'project:TEST'\nstate: 'synced'\n" +
			"remote_version: '1'\nupdated: '" + updated + "'\nsummary: 'original'\n---\n## Description\nbody\n\n"
		return schema.StampSyncedHash(doc)
	}
	write := func(key, content string) {
		if err := os.WriteFile(filepath.Join(local, key+".md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("TEST-1", synced("TEST-1", "2026-01-01T00:00:00Z"))
	write("TEST-2", strings.Replace(synced("TEST-2", "2026-01-01T00:00:00Z"), "original", "edited", 1))
	write("TEST-3", "---\nkey: 'TEST-3'\ntype: 'story'\nproject: 'project:TEST'\nstate: 'draft'\nsummary: 'new thing'\n---\n")
	write("TEST-4", strings.Replace(synced("TEST-4", "2026-01-01T00:00:00Z"), "original", "edited", 1))
	write("notes", "not an issue, no frontmatter\n")

	mirrorPath := filepath.Join(tmpDir, "mirror", "mirror.yml")
	data, err := os.ReadFile(mirrorPath)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("remote_updated:\n  TEST-4: '2026-02-01T00:00:00Z'\n  TEST-1: '2026-02-01T00:00:00Z'\n")...)
	if err := os.WriteFile(mirrorPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(local)
	return local
}

func TestRunStatus_PorcelainGolden(t *testing.T) {
	local := changesFixture(t)
	stdout, stderr := withStatusTestIO(t)
	if exit := RunStatus([]string{"--porcelain"}); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr.String())
	}
	want := "conflicted\tTEST-4\t" + filepath.Join(local, "TEST-4.md") + "\n" +
		"modified\tTEST-2\t" + filepath.Join(local, "TEST-2.md") + "\n" +
		"draft\tTEST-3\t" + filepath.Join(local, "TEST-3.md") + "\n"
	if got := stdout.String(); got != want {
		t.Fatalf("porcelain output:\n%q\nwant:\n%q", got, want)
	}
}

func TestRunStatus_ListsAllGroups(t *testing.T) {
	local := changesFixture(t)
	stdout, stderr := withStatusTestIO(t)
	if exit := RunStatus(nil); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr.String())
	}
	out := stdout.String()
	t.Logf("status output:\n%s", out)
	for _, want := range []string{
		"Local changes:",
		"  conflicted:\n    TEST-4  " + filepath.Join(local, "TEST-4.md") + "\n      run: jirafs diff TEST-4\n",
		"  modified:\n    TEST-2  " + filepath.Join(local, "TEST-2.md") + "\n      run: jirafs diff TEST-2\n",
		"  draft:\n    TEST-3  " + filepath.Join(local, "TEST-3.md") + "\n      run: jirafs push " + filepath.Join(local, "TEST-3.md") + "\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(out, "TEST-1") || strings.Contains(out, "no local changes") {
		t.Errorf("clean file or clean message wrongly present:\n%s", out)
	}
}

func TestRunStatus_CleanTree(t *testing.T) {
	local := changesFixture(t)
	for _, k := range []string{"TEST-2", "TEST-3", "TEST-4"} {
		if err := os.Remove(filepath.Join(local, k+".md")); err != nil {
			t.Fatal(err)
		}
	}
	stdout, stderr := withStatusTestIO(t)
	if exit := RunStatus(nil); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no local changes") {
		t.Fatalf("expected 'no local changes', got:\n%s", stdout.String())
	}
	stdout.Reset()
	RunStatus([]string{"--porcelain"})
	if stdout.String() != "" {
		t.Fatalf("clean porcelain must be empty, got %q", stdout.String())
	}
}

func TestRemoteAhead(t *testing.T) {
	cases := []struct {
		remote, local string
		want          bool
	}{
		{"2026-02-01T00:00:00Z", "2026-01-01T00:00:00Z", true},
		{"2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", false},
		{"2026-01-01T00:00:00.000+0000", "2025-12-31T00:00:00.000+0000", true},
		{"", "2026-01-01T00:00:00Z", false},
		{"2026-01-01T00:00:00Z", "", false},
		{"b", "a", true},
	}
	for _, c := range cases {
		if got := remoteAhead(c.remote, c.local); got != c.want {
			t.Errorf("remoteAhead(%q,%q) = %v, want %v", c.remote, c.local, got, c.want)
		}
	}
}

func TestWriteSyncedIssue_StampsHashAndRecordsRemote(t *testing.T) {
	tmpDir := t.TempDir()
	writeMirror(t, tmpDir)
	mirrorDir := filepath.Join(tmpDir, "mirror")
	dest := filepath.Join(tmpDir, "TEST-9.md")
	issue := &schema.Issue{Identity: schema.IssueIdentity{Key: "TEST-9", Type: "story"}, Summary: "s"}
	issue.Context.Updated = "2026-03-01T00:00:00Z"
	if err := writeSyncedIssue(dest, mirrorDir, issue); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dest)
	if schema.SyncedHash(string(data)) != schema.ContentHash(string(data)) || schema.SyncedHash(string(data)) == "" {
		t.Fatalf("file not stamped consistently:\n%s", data)
	}
	m, _, err := loadMirrorYAML(mirrorDir)
	if err != nil {
		t.Fatal(err)
	}
	if m.RemoteUpdatedFor("TEST-9") != "2026-03-01T00:00:00Z" {
		t.Fatalf("remote_updated not recorded: %v", m.RemoteUpdated)
	}
}
