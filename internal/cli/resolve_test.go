package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jirafs/jirafs/internal/merge"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

func withResolveIO(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	oldOut, oldErr, oldNow := resolveStdout, resolveStderr, resolveNow
	resolveStdout, resolveStderr = out, errb
	resolveNow = func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { resolveStdout, resolveStderr, resolveNow = oldOut, oldErr, oldNow })
	return out, errb
}

func conflictedIssue(summary, desc string) schema.Issue {
	return schema.Issue{
		Identity:       schema.IssueIdentity{Key: "TEST-7", Type: "story", Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}},
		RemoteMetadata: schema.RemoteMetadata{StateFile: "synced"},
		Summary:        summary,
		Description:    desc,
	}
}

// resolveFixture writes TEST-7.md with two recorded conflicts (summary and
// description; local values "mine", remote values "theirs") and a base snapshot
// holding the old values. It returns the local file and the mirror dir.
func resolveFixture(t *testing.T) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmp))
	writeMirror(t, tmp)
	local := filepath.Join(tmp, "local")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	mirrorDir := filepath.Join(tmp, "mirror")
	remote := conflictedIssue("theirs summary", "theirs description")
	base := conflictedIssue("old summary", "old description")
	if err := mirror.SaveBase(mirrorDir, &base); err != nil {
		t.Fatal(err)
	}
	mine := conflictedIssue("mine summary", "mine description")
	doc := schema.StampSyncedHashAs(schema.RenderIssue(mine), schema.ContentHash(schema.RenderIssue(remote)))
	doc = schema.AppendFrontmatter(doc, conflictsBlock([]merge.Conflict{
		{Field: schema.EditableFieldSummary, Remote: "theirs summary", Base: "old summary"},
		{Field: schema.EditableFieldDescription, Remote: "theirs description", Base: "old description"},
	}))
	path := filepath.Join(local, "TEST-7.md")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(local)
	return path, mirrorDir
}

func TestStatusShowsRecordedConflict(t *testing.T) {
	path, _ := resolveFixture(t)
	stdout, stderr := withStatusTestIO(t)
	if exit := RunStatus(nil); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr)
	}
	t.Logf("status:\n%s", stdout)
	for _, want := range []string{"conflicted:", "TEST-7  " + path, "fields: summary, description", "review: jirafs diff TEST-7", "run: jirafs resolve TEST-7 --keep local|remote [--field NAME]"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("missing %q in:\n%s", want, stdout)
		}
	}
	stdout.Reset()
	if exit := RunStatus([]string{"--porcelain"}); exit != 0 {
		t.Fatalf("porcelain exit %d", exit)
	}
	if want := "conflicted\tTEST-7\t" + path + "\n"; stdout.String() != want {
		t.Errorf("porcelain = %q, want %q", stdout, want)
	}
}

func TestPushRefusesRecordedConflict(t *testing.T) {
	resolveFixture(t)
	stdout, _ := withPushIO(t)
	if exit := RunPush([]string{"--project", "test"}); exit != 0 {
		t.Fatalf("an implicit run skips conflicts without failing; exit = %d\n%s", exit, stdout)
	}
	for _, want := range []string{"TEST-7  skipped (conflict)", "unresolved conflict on summary, description", "jirafs resolve TEST-7 --keep local|remote"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("missing %q in:\n%s", want, stdout)
		}
	}
	t.Logf("implicit push:\n%s", stdout)

	stdout.Reset()
	if exit := RunPush([]string{"--project", "test", "--apply", "TEST-7"}); exit != 1 {
		t.Fatalf("explicit --apply on a conflicted file must exit 1, got %d\n%s", exit, stdout)
	}
	if !strings.Contains(stdout.String(), "TEST-7  skipped (conflict)") {
		t.Errorf("summary:\n%s", stdout)
	}
	t.Logf("explicit push --apply:\n%s", stdout)

	stdout.Reset()
	if exit := RunPush([]string{"--project", "test", "TEST-7"}); exit != 0 {
		t.Errorf("dry run of a named conflicted file exits 0, got %d", exit)
	}
}

func TestResolveDryRunWritesNothing(t *testing.T) {
	path, _ := resolveFixture(t)
	before := readFile(t, path)
	stdout, _ := withResolveIO(t)
	if exit := RunResolve([]string{"TEST-7", "--keep", "remote", "--project", "test"}); exit != 0 {
		t.Fatalf("exit %d", exit)
	}
	if readFile(t, path) != before {
		t.Error("dry run changed the file")
	}
	for _, want := range []string{"would resolve TEST-7: keep remote for summary, description", "dry run: nothing was written; to run it for real: jirafs resolve TEST-7 --keep remote --apply --project test"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("missing %q in:\n%s", want, stdout)
		}
	}
	t.Logf("resolve dry run:\n%s", stdout)
}

func TestResolveKeepLocal(t *testing.T) {
	path, mirrorDir := resolveFixture(t)
	stdout, stderr := withResolveIO(t)
	if exit := RunResolve([]string{"TEST-7", "--keep", "local", "--apply", "--project", "test"}); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr)
	}
	t.Logf("resolve --keep local:\n%s", stdout)
	doc := readFile(t, path)
	if strings.Contains(doc, "conflicts:") || docConflicts(doc) != nil {
		t.Errorf("conflicts key not cleared:\n%s", doc)
	}
	issue, pe := schema.ParseIssue(doc)
	if pe != nil || issue.Summary != "mine summary" || issue.Description != "mine description" {
		t.Errorf("local values lost: %+v %v", issue, pe)
	}
	base, ok := mirror.LoadBase(mirrorDir, "TEST-7")
	if !ok || base.Summary != "theirs summary" || base.Description != "theirs description" {
		t.Errorf("base must advance to the remote value so push can send the local one: %+v", base)
	}
	if entries, _ := os.ReadDir(mirror.BackupDir(mirrorDir)); len(entries) != 0 {
		t.Errorf("keep local must not need a backup: %v", entries)
	}
	if !strings.Contains(stdout.String(), "no conflicts left on TEST-7") {
		t.Errorf("next step missing:\n%s", stdout)
	}
	// The file is a plain local edit now: status reports modified, push proceeds.
	statusOut, _ := withStatusTestIO(t)
	RunStatus([]string{"--porcelain"})
	if !strings.HasPrefix(statusOut.String(), "modified\tTEST-7\t") {
		t.Errorf("status after resolve = %q", statusOut)
	}
}

func TestResolveKeepRemoteSavesBackup(t *testing.T) {
	path, mirrorDir := resolveFixture(t)
	before := readFile(t, path)
	stdout, stderr := withResolveIO(t)
	if exit := RunResolve([]string{"TEST-7", "--keep", "remote", "--apply", "--project", "test"}); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr)
	}
	t.Logf("resolve --keep remote:\n%s", stdout)
	doc := readFile(t, path)
	issue, pe := schema.ParseIssue(doc)
	if pe != nil || issue.Summary != "theirs summary" || issue.Description != "theirs description" {
		t.Errorf("remote values not taken: %+v %v\n%s", issue, pe, doc)
	}
	if strings.Contains(doc, "conflicts:") {
		t.Errorf("conflicts key not cleared:\n%s", doc)
	}
	backups, _ := filepath.Glob(filepath.Join(mirror.BackupDir(mirrorDir), "TEST-7-*.md"))
	if len(backups) != 1 || readFile(t, backups[0]) != before {
		t.Fatalf("backup must hold the pre-resolve local file: %v", backups)
	}
	if !strings.Contains(stdout.String(), "local file saved to "+backups[0]) {
		t.Errorf("backup path not reported:\n%s", stdout)
	}
	base, _ := mirror.LoadBase(mirrorDir, "TEST-7")
	if base == nil || base.Summary != "theirs summary" {
		t.Errorf("base = %+v", base)
	}
	statusOut, _ := withStatusTestIO(t)
	RunStatus([]string{"--porcelain"})
	if statusOut.String() != "" {
		t.Errorf("file equals the remote now, status = %q", statusOut)
	}
}

func TestResolvePerFieldKeepsTheRest(t *testing.T) {
	path, mirrorDir := resolveFixture(t)
	stdout, stderr := withResolveIO(t)
	if exit := RunResolve([]string{"TEST-7", "--keep", "remote", "--field", "summary", "--apply", "--project", "test"}); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr)
	}
	t.Logf("partial resolve:\n%s", stdout)
	doc := readFile(t, path)
	issue, _ := schema.ParseIssue(doc)
	if issue.Summary != "theirs summary" || issue.Description != "mine description" {
		t.Errorf("summary=%q description=%q", issue.Summary, issue.Description)
	}
	left := docConflicts(doc)
	if len(left) != 1 || left[0].Field != "description" || left[0].Remote != "theirs description" || left[0].Base != "old description" {
		t.Errorf("remaining = %+v\n%s", left, doc)
	}
	base, _ := mirror.LoadBase(mirrorDir, "TEST-7")
	if base.Summary != "theirs summary" || base.Description != "old description" {
		t.Errorf("only the resolved field may advance in the base: %q / %q", base.Summary, base.Description)
	}
	if !strings.Contains(stdout.String(), "1 conflict(s) left on TEST-7: description") {
		t.Errorf("remaining conflicts not reported:\n%s", stdout)
	}
	// Still blocked for push.
	pushOut, _ := withPushIO(t)
	if exit := RunPush([]string{"--project", "test", "--apply", "TEST-7"}); exit != 1 || !strings.Contains(pushOut.String(), "unresolved conflict on description") {
		t.Errorf("push exit %d:\n%s", exit, pushOut)
	}
	// Resolving the last one clears the key.
	withResolveIO(t)
	if exit := RunResolve([]string{"TEST-7", "--keep", "local", "--field", "description", "--apply", "--project", "test"}); exit != 0 {
		t.Fatalf("second resolve exit %d", exit)
	}
	if doc := readFile(t, path); strings.Contains(doc, "conflicts") {
		t.Errorf("conflicts key remains:\n%s", doc)
	}
}

func TestResolveWithoutBaseBuildsOne(t *testing.T) {
	path, mirrorDir := resolveFixture(t)
	if err := os.RemoveAll(mirror.BaseDir(mirrorDir)); err != nil {
		t.Fatal(err)
	}
	withResolveIO(t)
	if exit := RunResolve([]string{"TEST-7", "--keep", "local", "--field", "summary", "--apply", "--project", "test"}); exit != 0 {
		t.Fatalf("exit %d", exit)
	}
	base, ok := mirror.LoadBase(mirrorDir, "TEST-7")
	if !ok || base.Summary != "theirs summary" || base.Description != "theirs description" {
		t.Errorf("base = %+v", base)
	}
	if left := docConflicts(readFile(t, path)); len(left) != 1 {
		t.Errorf("remaining = %+v", left)
	}
}

func TestResolveErrors(t *testing.T) {
	path, _ := resolveFixture(t)
	before := readFile(t, path)
	cases := []struct {
		name string
		args []string
		exit int
		want string
	}{
		{"no key", []string{"--keep", "local"}, 2, "expected exactly one issue key"},
		{"no keep", []string{"TEST-7"}, 2, "--keep must be local or remote"},
		{"bad keep", []string{"TEST-7", "--keep", "both"}, 2, "--keep must be local or remote"},
		{"dry and apply", []string{"TEST-7", "--keep", "local", "--dry-run", "--apply"}, 2, "cannot be used together"},
		{"bad flag", []string{"--nope"}, 2, "flag provided but not defined"},
		{"unknown issue", []string{"TEST-99", "--keep", "local", "--project", "test"}, 1, "not found"},
		{"unknown field", []string{"TEST-7", "--keep", "local", "--field", "labels", "--project", "test"}, 1, "no conflict on field labels; conflicted fields: summary, description"},
		{"no project", []string{"TEST-7", "--keep", "local", "--project", "nope"}, 1, "nope"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, stderr := withResolveIO(t)
			if exit := RunResolve(c.args); exit != c.exit || !strings.Contains(stderr.String(), c.want) {
				t.Errorf("exit %d (want %d), stderr %q (want %q)", exit, c.exit, stderr, c.want)
			}
		})
	}
	if readFile(t, path) != before {
		t.Error("an error changed the file")
	}

	t.Run("no conflicts", func(t *testing.T) {
		if err := os.WriteFile(path, []byte(schema.RenderIssue(conflictedIssue("a", "b"))), 0o644); err != nil {
			t.Fatal(err)
		}
		_, stderr := withResolveIO(t)
		if exit := RunResolve([]string{"TEST-7", "--keep", "local", "--project", "test"}); exit != 1 || !strings.Contains(stderr.String(), "has no recorded conflicts") {
			t.Errorf("exit %d: %s", exit, stderr)
		}
	})
}

func TestResolveHelp(t *testing.T) {
	stdout, _ := withResolveIO(t)
	if exit := RunResolve([]string{"--help"}); exit != 0 || !strings.Contains(stdout.String(), "--keep local|remote") {
		t.Errorf("exit %d:\n%s", exit, stdout)
	}
}

func TestApplyRemoteValueFields(t *testing.T) {
	issue := schema.Issue{Labels: []string{"a"}, CustomFields: map[string]interface{}{"cf1": "x", "cf2": "y", "cf3": "z"}}
	for _, e := range []conflictEntry{
		{Field: "labels", Remote: "b,c"},
		{Field: "fix_versions", Remote: "1.0"},
		{Field: "assignee", Remote: "bob"},
		{Field: "sprint", Remote: "S1"},
		{Field: "status", Remote: "Done"},
		{Field: "priority", Remote: "High"},
		{Field: "linked_issues", Remote: "T-1|blocks|Blocks|outward\nT-2|||"},
		{Field: "custom_fields", Remote: `{"cf1":"new"}`, Base: `{"cf1":"x","cf2":"y"}`},
	} {
		if err := applyRemoteValue(&issue, e); err != nil {
			t.Fatalf("%s: %v", e.Field, err)
		}
	}
	if strings.Join(issue.Labels, ",") != "b,c" || strings.Join(issue.FixVersions, ",") != "1.0" || *issue.Assignee != "bob" ||
		issue.Sprint != "S1" || issue.Status != "Done" || issue.Context.Status != "Done" || issue.Context.Priority != "High" ||
		len(issue.LinkedIssues) != 2 || issue.LinkedIssues[0].LinkType != "Blocks" || issue.LinkedIssues[1].Key != "T-2" {
		t.Errorf("issue = %+v", issue)
	}
	if len(issue.CustomFields) != 2 || issue.CustomFields["cf1"] != "new" || issue.CustomFields["cf3"] != "z" {
		t.Errorf("custom fields = %v", issue.CustomFields)
	}
	// Empty remote values clear the field.
	for _, f := range []string{"labels", "assignee", "linked_issues", "description"} {
		if err := applyRemoteValue(&issue, conflictEntry{Field: f}); err != nil {
			t.Fatal(err)
		}
	}
	if issue.Labels != nil || issue.Assignee != nil || issue.LinkedIssues != nil {
		t.Errorf("not cleared: %+v", issue)
	}
	if err := applyRemoteValue(&issue, conflictEntry{Field: "custom_fields", Remote: `{"cf1"`}); err == nil {
		t.Error("bad remote JSON accepted")
	}
	if err := applyRemoteValue(&issue, conflictEntry{Field: "custom_fields", Remote: `{}`, Base: `[`}); err == nil {
		t.Error("bad base JSON accepted")
	}
	if err := applyRemoteValue(&issue, conflictEntry{Field: "custom_fields", Remote: `{}`, Base: `{"cf1":1,"cf3":1}`}); err != nil || issue.CustomFields != nil {
		t.Errorf("custom fields = %v, err %v", issue.CustomFields, err)
	}
	if err := applyRemoteValue(&issue, conflictEntry{Field: "wat"}); err == nil {
		t.Error("unknown field accepted")
	}
}

func TestDocConflictsIgnoresNoise(t *testing.T) {
	for _, doc := range []string{"plain", "---\nkey: A-1\n---\n", "---\n: [\n---\n"} {
		if got := docConflicts(doc); got != nil {
			t.Errorf("docConflicts(%q) = %v", doc, got)
		}
	}
}
