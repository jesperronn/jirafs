package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var registryTypeCases = []struct {
	typ, file, key, name string
	entry                string
}{
	{"users", "users.yaml", "user:amber", "Amber User", "    display_name: \"Amber User\"\n"},
	{"projects", "projects.yaml", "project:abc", "ABC", "    key: ABC\n    name: Alpha\n    id: \"1\"\n"},
	{"statuses", "statuses.yaml", "status:todo", "To Do", "    name: To Do\n"},
	{"sprints", "sprints.yaml", "sprint:s1", "Sprint 1", "    name: Sprint 1\n"},
	{"fix_versions", "fix_versions.yaml", "fix_version:v1", "1.0.0", "    name: 1.0.0\n"},
}

func TestRunRegistry_AllTypes_InitShowJSON(t *testing.T) {
	for _, tc := range registryTypeCases {
		t.Run(tc.typ, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("HOME", writeRegistrySettings(t, tmpDir))
			mirrorDir := filepath.Join(tmpDir, "mirror")

			stdout, stderr := withRegistryTestIO(t)
			if exit := RunRegistry([]string{"init", tc.typ, "--project", "test"}); exit != 0 {
				t.Fatalf("init = %d, stderr %q", exit, stderr.String())
			}
			path := filepath.Join(mirrorDir, tc.file)
			if !strings.Contains(stdout.String(), "created "+path) {
				t.Fatalf("init stdout = %q", stdout.String())
			}
			stdout.Reset()
			if exit := RunRegistry([]string{"init", tc.typ, "--project", "test"}); exit != 0 || !strings.Contains(stdout.String(), "already exists") {
				t.Fatalf("second init = %d, stdout %q", exit, stdout.String())
			}
			stdout.Reset()
			if exit := RunRegistry([]string{"show", tc.typ, "--project", "test"}); exit != 0 || !strings.Contains(stdout.String(), "(empty)") {
				t.Fatalf("show empty = %d, stdout %q", exit, stdout.String())
			}

			content := "entries:\n  \"" + tc.key + "\":\n" + tc.entry
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			stdout.Reset()
			if exit := RunRegistry([]string{"show", tc.typ, "--project", "test"}); exit != 0 {
				t.Fatalf("show = %d, stderr %q", exit, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.key) || !strings.Contains(stdout.String(), tc.name) {
				t.Fatalf("show stdout = %q, want key %q and summary %q", stdout.String(), tc.key, tc.name)
			}
			stdout.Reset()
			if exit := RunRegistry([]string{"show", tc.typ, "--json", "--project", "test"}); exit != 0 || !strings.Contains(stdout.String(), tc.key) {
				t.Fatalf("show json = %d, stdout %q", exit, stdout.String())
			}

			// Malformed YAML is reported, not swallowed.
			if err := os.WriteFile(path, []byte("entries: [unterminated\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			stderr.Reset()
			if exit := RunRegistry([]string{"show", tc.typ, "--project", "test"}); exit != 1 || !strings.Contains(stderr.String(), "jirafs registry show "+tc.typ) {
				t.Fatalf("show malformed = %d, stderr %q", exit, stderr.String())
			}
		})
	}
}

func TestRunRegistry_ShowInit_ArgErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", writeRegistrySettings(t, tmpDir))
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"show", "--bogus"}, "flag provided but not defined"},
		{[]string{"show"}, "missing <type>"},
		{[]string{"init", "--bogus"}, "flag provided but not defined"},
		{[]string{"init"}, "missing <type>"},
		{[]string{"init", "nope", "--project", "test"}, "unknown type"},
	}
	for _, c := range cases {
		_, stderr := withRegistryTestIO(t)
		if exit := RunRegistry(c.args); exit != wantExit(c.want) || !strings.Contains(stderr.String(), c.want) {
			t.Errorf("RunRegistry(%v) = %d, stderr %q, want %q", c.args, exit, stderr.String(), c.want)
		}
	}
}

func TestRunRegistry_InitNoSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, stderr := withRegistryTestIO(t)
	if exit := RunRegistry([]string{"init", "users"}); exit != 1 || !strings.Contains(stderr.String(), "no settings found") {
		t.Fatalf("exit/stderr = %d %q", exit, stderr.String())
	}
}

// wantExit is 1 for runtime failures (unknown registry type) and 2 for usage errors.
func wantExit(want string) int {
	if want == "unknown type" {
		return 1
	}
	return 2
}
