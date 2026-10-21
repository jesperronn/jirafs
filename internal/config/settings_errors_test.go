package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRawSettings(t *testing.T, content string) (home, path string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".jirafs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, "settings.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return home, path
}

func TestSaveState_InvalidExistingTOML(t *testing.T) {
	_, path := writeRawSettings(t, "this is = not [valid")
	s := &Settings{}
	err := s.SaveState()
	if err == nil || !strings.Contains(err.Error(), "invalid TOML") {
		t.Fatalf("err = %v, want invalid TOML", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "this is = not [valid" {
		t.Errorf("file was modified on error: %q", data)
	}
}

func TestSetupProject_InvalidExistingTOML(t *testing.T) {
	writeRawSettings(t, "= broken")
	s := &Settings{}
	err := s.SetupProject("i", "p", "KEY", "https://x.atlassian.net", "basic", t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "invalid TOML") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadOrCreate_UnreadableFile(t *testing.T) {
	home, path := writeRawSettings(t, "")
	// Replace the file with a directory so ReadFile fails with a non-NotExist error.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = home
	_, err := (&Settings{}).loadOrCreate()
	if err == nil || !strings.Contains(err.Error(), "cannot read settings file") {
		t.Fatalf("err = %v", err)
	}
	if err := (&Settings{}).SaveState(); err == nil {
		t.Error("SaveState should fail when settings path is a directory")
	}
}

func TestEnsureSettingsDir_CannotCreate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// A regular file where ~/.jirafs should be makes MkdirAll fail.
	if err := os.WriteFile(filepath.Join(home, ".jirafs"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	err := ensureSettingsDirExists()
	if err == nil || !strings.Contains(err.Error(), "cannot create settings directory") {
		t.Fatalf("err = %v", err)
	}
	err = (&Settings{}).SetupProject("i", "p", "KEY", "https://x.atlassian.net", "basic", filepath.Join(home, "m"), nil)
	if err == nil {
		t.Error("SetupProject should surface the directory creation failure")
	}
}

func TestSetupProject_CreatesLocalDirsDefaultAndKeepsExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mirror := filepath.Join(home, "m")
	s := &Settings{}
	if err := s.SetupProject("i", "p", "KEY", "https://x.atlassian.net", "basic", mirror, []string{"env:TOKEN"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	proj := loaded.Projects["p"]
	if len(proj.LocalDirs) != 1 || proj.LocalDirs[0] != mirror {
		t.Errorf("LocalDirs = %v, want [%s]", proj.LocalDirs, mirror)
	}
	if got := loaded.Instances["i"].CredentialRefs; len(got) != 1 || got[0] != "env:TOKEN" {
		t.Errorf("CredentialRefs = %v", got)
	}
}

func TestExpandPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("JF_TEST_DIR", "/data")
	cases := map[string]string{
		"":                  "",
		"~":                 home,
		"~/x/y":             filepath.Join(home, "x/y"),
		"$JF_TEST_DIR/a":    "/data/a",
		"${JF_TEST_DIR}/b":  "/data/b",
		"/plain/path":       "/plain/path",
		"$JF_UNSET_VAR_XYZ": "",
	}
	for in, want := range cases {
		got, err := expandPath(in)
		if err != nil || got != want {
			t.Errorf("expandPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
