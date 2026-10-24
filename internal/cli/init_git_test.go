package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/gitx"
	"github.com/jirafs/jirafs/internal/mirror"
)

func gitInitSetup(t *testing.T) {
	t.Helper()
	gitx.RequireGit(t)
	gitx.HermeticEnv(t)
	useInitHome(t)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertGitMirror(t *testing.T, dir string) {
	t.Helper()
	m, err := mirror.UnmarshalMirror([]byte(readFile(t, filepath.Join(dir, "mirror.yml"))))
	if err != nil || !m.Git {
		t.Fatalf("mirror.yml git = %v, err = %v", m != nil && m.Git, err)
	}
	ign := readFile(t, filepath.Join(dir, ".gitignore"))
	for _, e := range []string{".jirafs/backup/", ".jirafs/intents/", ".jirafs/tmp/", ".jirafs/base/"} {
		if strings.Count(ign, e+"\n") != 1 {
			t.Fatalf(".gitignore lacks exactly one %q: %q", e, ign)
		}
	}
}

func TestRunInitGit_EmptyDir(t *testing.T) {
	gitInitSetup(t)
	dir := t.TempDir()
	_, stderr := withInitTestIO(t)
	if exit := RunInit(initArgs(dir, "--git")); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("no .git: %v", err)
	}
	if !strings.Contains(stderr.String(), "created new git repository") {
		t.Fatalf("stderr: %s", stderr)
	}
	assertGitMirror(t, dir)
}

func TestRunInitGit_DefaultOff(t *testing.T) {
	gitInitSetup(t)
	dir := t.TempDir()
	withInitTestIO(t)
	if exit := RunInit(initArgs(dir)); exit != 0 {
		t.Fatal(exit)
	}
	if readFile(t, filepath.Join(dir, "mirror.yml")) != wantMirrorYAML {
		t.Fatal("mirror.yml changed without --git")
	}
	for _, p := range []string{".git", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			t.Fatalf("%s created without --git", p)
		}
	}
}

func TestRunInitGit_ExistingRepoAndGitignoreAppended(t *testing.T) {
	gitInitSetup(t)
	dir := t.TempDir()
	if err := gitx.Init(dir); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log"), 0o644)
	_, stderr := withInitTestIO(t)
	if exit := RunInit(initArgs(dir, "--git")); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr)
	}
	if !strings.Contains(stderr.String(), "reusing existing git repository") {
		t.Fatalf("stderr: %s", stderr)
	}
	ign := readFile(t, filepath.Join(dir, ".gitignore"))
	if !strings.HasPrefix(ign, "*.log\n.jirafs/backup/\n") {
		t.Fatalf(".gitignore = %q", ign)
	}
	assertGitMirror(t, dir)
}

func TestRunInitGit_SubdirOfParentRepo(t *testing.T) {
	gitInitSetup(t)
	parent := t.TempDir()
	if err := gitx.Init(parent); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(parent, "jira", "plat")
	_, stderr := withInitTestIO(t)
	if exit := RunInit(initArgs(dir, "--git")); exit != 0 {
		t.Fatalf("exit %d: %s", exit, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		t.Fatal("nested repo created inside parent repo")
	}
	if !strings.Contains(stderr.String(), "reusing existing git repository") {
		t.Fatalf("stderr: %s", stderr)
	}
	assertGitMirror(t, dir)
}

func TestRunInitGit_Idempotent(t *testing.T) {
	gitInitSetup(t)
	dir := t.TempDir()
	withInitTestIO(t)
	for i := 0; i < 2; i++ {
		if exit := RunInit(initArgs(dir, "--git")); exit != 0 {
			t.Fatalf("run %d exit %d", i, exit)
		}
	}
	assertGitMirror(t, dir)
	if n := strings.Count(readFile(t, filepath.Join(dir, "mirror.yml")), "git: true"); n != 1 {
		t.Fatalf("git: true appears %d times", n)
	}
}

func TestRunInitGit_ExistingMirrorYamlWithoutTrailingNewline(t *testing.T) {
	gitInitSetup(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mirror.yaml"), []byte("project:\n  type: project\n  value: PLAT"), 0o644)
	withInitTestIO(t)
	if exit := RunInit(initArgs(dir, "--git")); exit != 0 {
		t.Fatal(exit)
	}
	m, err := mirror.UnmarshalMirror([]byte(readFile(t, filepath.Join(dir, "mirror.yaml"))))
	if err != nil || !m.Git {
		t.Fatalf("git not set: %v", err)
	}
}

func TestRunInitGit_MissingGit(t *testing.T) {
	useInitHome(t)
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	_, stderr := withInitTestIO(t)
	if exit := RunInit(initArgs(dir, "--git")); exit != 1 {
		t.Fatalf("exit %d", exit)
	}
	want := "jirafs: git is not installed or not on PATH; install it (brew install git / apt install git) or run: jirafs init"
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "mirror.yml")); err == nil {
		t.Fatal("init wrote files despite missing git")
	}
}

func TestInitGitHelpers_Errors(t *testing.T) {
	dir := t.TempDir()
	if err := ensureGitSetting(dir); err == nil {
		t.Fatal("expected error when mirror.yml missing")
	}
	os.WriteFile(filepath.Join(dir, "mirror.yml"), []byte("project: [unclosed"), 0o644)
	if err := ensureGitSetting(dir); err == nil {
		t.Fatal("expected parse error")
	}
	// .gitignore as a directory cannot be read or written.
	os.Mkdir(filepath.Join(dir, ".gitignore"), 0o755)
	if err := ensureGitignore(dir); err == nil {
		t.Fatal("expected gitignore error")
	}
	if err := initGitMirror(os.Stderr, filepath.Join(dir, "nope")); err == nil {
		t.Fatal("expected error for missing dir")
	}
}
