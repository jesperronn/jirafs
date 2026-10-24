package gitx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRepoLifecycle(t *testing.T) {
	RequireGit(t)
	HermeticEnv(t)
	if !Available() {
		t.Fatal("Available false")
	}
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Fatal("temp dir should not be a repo")
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(dir) {
		t.Fatal("expected repo")
	}
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0o755)
	top, err := Toplevel(sub)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := filepath.EvalSymlinks(top); a != func() string { b, _ := filepath.EvalSymlinks(dir); return b }() {
		t.Fatalf("toplevel %q", top)
	}
	write(t, filepath.Join(dir, "a.md"), "a")
	write(t, filepath.Join(dir, "b.md"), "b")
	st, err := Status(dir)
	if err != nil || len(st) != 2 || st[0].XY != "??" {
		t.Fatalf("status = %v %v", st, err)
	}
	if st, _ := Status(dir, "a.md"); len(st) != 1 || st[0].Path != "a.md" {
		t.Fatalf("scoped status = %v", st)
	}
	if err := Commit(dir, []string{"a.md"}, "add a"); err != nil {
		t.Fatal(err)
	}
	if st, _ := Status(dir); len(st) != 1 || st[0].Path != "b.md" {
		t.Fatalf("after commit = %v", st)
	}
	if err := os.Rename(filepath.Join(dir, "a.md"), filepath.Join(dir, "c.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(dir, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	st, err = Status(dir)
	if err != nil || len(st) != 2 {
		t.Fatalf("rename status = %v %v", st, err)
	}
	if err := Commit(dir, []string{"nonexistent-xyz"}, "x"); err == nil {
		t.Fatal("expected error committing unknown path")
	}
	if _, err := Status(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected status error in missing dir")
	}
	if _, err := Toplevel(t.TempDir()); err == nil {
		t.Fatal("expected toplevel error outside repo")
	}
}

func TestCommitNoIdentity(t *testing.T) {
	RequireGit(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")
	t.Setenv("EMAIL", "")
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "a.md"), "a")
	// Disable auto-detected identity so git must complain.
	if _, err := Run(dir, "config", "user.useConfigOnly", "true"); err != nil {
		t.Fatal(err)
	}
	err := Commit(dir, []string{"a.md"}, "m")
	if !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("err = %v, want ErrNoIdentity", err)
	}
	if !strings.Contains(err.Error(), `git config --global user.name "Your Name"`) ||
		!strings.Contains(err.Error(), "git config --global user.email you@example.com") {
		t.Fatalf("hint missing: %v", err)
	}
}

func TestMissingGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if Available() {
		t.Fatal("Available true with empty PATH")
	}
	if _, err := Run(t.TempDir(), "status"); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("err = %v", err)
	}
	if IsRepo(t.TempDir()) {
		t.Fatal("IsRepo true without git")
	}
	if !strings.Contains(ErrNotInstalled.Error(), "brew install git / apt install git") {
		t.Fatal("hint text")
	}
}

func TestRequireGitFailsWhenRequired(t *testing.T) {
	// With git present RequireGit must simply return.
	RequireGit(t)
}
