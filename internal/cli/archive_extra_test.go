package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListArchive_FiltersAndSorts(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"B-2.md", "A-1.md", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, _ := withArchiveTestIO(t)
	if code := listArchive(dir); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if got := stdout.String(); got != "A-1.md\nB-2.md\n" {
		t.Errorf("stdout = %q, want sorted .md files only", got)
	}
}

func TestListArchive_OnlyNonMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _ := withArchiveTestIO(t)
	if code := listArchive(dir); code != 0 || !strings.Contains(stdout.String(), "no archived issues") {
		t.Errorf("code=%d stdout=%q", code, stdout.String())
	}
}

func TestListArchive_NotADirectory(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr := withArchiveTestIO(t)
	if code := listArchive(f); code != 1 || !strings.Contains(stderr.String(), "jirafs archive list:") {
		t.Errorf("code=%d stderr=%q", code, stderr.String())
	}
}

func writeNoLocalDirsSettings(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".jirafs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	toml := `version = 1

[instances.default]
base_url = "https://example.atlassian.net"
auth_type = "basic"

[projects.test]
key = "TEST"
instance = "default"
mirror_dir = "` + filepath.Join(home, "mirror") + `"
`
	if err := os.WriteFile(filepath.Join(dir, "settings.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
}

func TestRunArchive_NoLocalDirs(t *testing.T) {
	writeNoLocalDirsSettings(t)
	for _, args := range [][]string{
		{"issue", "ABC-1", "--project", "test"},
		{"list", "--project", "test"},
	} {
		_, stderr := withArchiveTestIO(t)
		if code := RunArchive(args); code != 1 || !strings.Contains(stderr.String(), "no local directories configured") {
			t.Errorf("RunArchive(%v) = %d, stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRunArchive_Issue_NotFoundLocally(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	writeMirror(t, tmpDir)
	localDir := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, localDir)
	_, stderr := withArchiveTestIO(t)
	if code := RunArchive([]string{"issue", "ZZZ-9", "--project", "test"}); code != 1 || !strings.Contains(stderr.String(), "ZZZ-9 not found") {
		t.Errorf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunArchive_List_UnknownProject(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeSettings(t, tmpDir))
	_, stderr := withArchiveTestIO(t)
	if code := RunArchive([]string{"list", "--project", "nope"}); code != 1 || !strings.Contains(stderr.String(), "jirafs archive:") {
		t.Errorf("code=%d stderr=%q", code, stderr.String())
	}
}
