package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/codec"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello, World!":   "hello-world",
		"  --Foo__Bar  ":  "foo-bar",
		"!!!":             "new-issue",
		"":                "new-issue",
		"Already-slugged": "already-slugged",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRunNew_SummaryAssigneeAndSlug(t *testing.T) {
	localDir := setupNewTest(t)
	out, errBuf := withNewTestIO(t)
	code := RunNew([]string{"story", "--summary", "Fix The Thing!", "--assignee", "alice", "--cwd", localDir})
	if code != 0 {
		t.Fatalf("exit = %d stderr=%s", code, errBuf.String())
	}
	path := filepath.Join(localDir, "fix-the-thing.md")
	if !strings.Contains(out.String(), "created "+path) {
		t.Errorf("stdout = %q", out.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	issue, pe := codec.ParseIssue(string(data))
	if pe != nil {
		t.Fatalf("reparse: %v", pe)
	}
	if issue.Summary != "Fix The Thing!" {
		t.Errorf("Summary = %q", issue.Summary)
	}
	if issue.Assignee == nil || *issue.Assignee != "alice" {
		t.Errorf("Assignee = %v, want alice", issue.Assignee)
	}
}

func TestRunNew_ProjectFlagSetsProjectRef(t *testing.T) {
	localDir := setupNewTest(t)
	_, errBuf := withNewTestIO(t)
	code := RunNew([]string{"bug", "--summary", "Proj flag", "--project", "test", "--cwd", localDir})
	if code != 0 {
		t.Fatalf("exit = %d stderr=%s", code, errBuf.String())
	}
	data, err := os.ReadFile(filepath.Join(localDir, "proj-flag.md"))
	if err != nil {
		t.Fatal(err)
	}
	issue, pe := codec.ParseIssue(string(data))
	if pe != nil {
		t.Fatal(pe)
	}
	if issue.Identity.Project.Value != "TEST" {
		t.Errorf("project = %+v, want value TEST", issue.Identity.Project)
	}
}

func TestRunNew_Errors(t *testing.T) {
	localDir := setupNewTest(t)
	dir := t.TempDir()
	badTmpl := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(badTmpl, []byte("not a valid issue"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing template file", []string{"--template", filepath.Join(dir, "nope.md"), "--cwd", localDir}, "cannot read template"},
		{"template without summary", []string{"--template", badTmpl, "--cwd", localDir}, "summary is required"},
		{"unknown project", []string{"story", "--summary", "x", "--project", "nope", "--cwd", localDir}, "jirafs init"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, errBuf := withNewTestIO(t)
			if code := RunNew(c.args); code != 1 || !strings.Contains(errBuf.String(), c.want) {
				t.Errorf("code=%d stderr=%q, want %q", code, errBuf.String(), c.want)
			}
		})
	}
}

func TestRunNew_NoSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, errBuf := withNewTestIO(t)
	if code := RunNew([]string{"story", "--summary", "x"}); code != 1 || errBuf.Len() == 0 {
		t.Errorf("code=%d stderr=%q", code, errBuf.String())
	}
}

func TestRunNew_ProjectWithoutLocalDirs(t *testing.T) {
	writeNoLocalDirsSettings(t)
	_, errBuf := withNewTestIO(t)
	if code := RunNew([]string{"story", "--summary", "x", "--project", "test", "--cwd", t.TempDir()}); code != 1 || !strings.Contains(errBuf.String(), "jirafs init") {
		t.Errorf("code=%d stderr=%q", code, errBuf.String())
	}
}

func TestRunNew_WriteFails(t *testing.T) {
	localDir := setupNewTest(t)
	if err := os.Chmod(localDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(localDir, 0o755) })
	if f, err := os.Create(filepath.Join(localDir, "probe")); err == nil {
		f.Close()
		t.Skip("directory permissions not enforced (running as root?)")
	}
	_, errBuf := withNewTestIO(t)
	if code := RunNew([]string{"story", "--summary", "blocked", "--cwd", localDir}); code != 1 || !strings.Contains(errBuf.String(), "jirafs new:") {
		t.Errorf("code=%d stderr=%q", code, errBuf.String())
	}
}

func TestNewRefusesToOverwriteExistingDraft(t *testing.T) {
	localDir := setupNewTest(t)
	out, errBuf := withNewTestIO(t)
	args := []string{"story", "--summary", "Same name", "--cwd", localDir}
	if code := RunNew(args); code != 0 {
		t.Fatalf("first exit=%d err=%s", code, errBuf)
	}
	if !strings.Contains(out.String(), "next: edit it, then run: jirafs push "+filepath.Join(localDir, "same-name.md")) {
		t.Errorf("missing push hint: %s", out)
	}
	path := filepath.Join(localDir, "same-name.md")
	if err := os.WriteFile(path, []byte("hand edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	errBuf.Reset()
	if code := RunNew(args); code != 1 {
		t.Fatalf("second exit=%d, want 1", code)
	}
	for _, want := range []string{"already exists", "jirafs new story --summary 'Same name' --cwd " + localDir + " --force"} {
		if !strings.Contains(errBuf.String(), want) {
			t.Errorf("stderr missing %q:\n%s", want, errBuf)
		}
	}
	if data, _ := os.ReadFile(path); string(data) != "hand edited" {
		t.Fatalf("file was overwritten: %q", data)
	}
	if code := RunNew(append(args, "--force")); code != 0 {
		t.Fatalf("--force exit=%d", code)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "Same name") {
		t.Fatalf("--force did not overwrite: %q", data)
	}
}
