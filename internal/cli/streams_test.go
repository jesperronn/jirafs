package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/registry"
)

// swapStreams points every command's stdout/stderr at fresh buffers and
// restores the originals on cleanup.
func swapStreams(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	stdouts := []*io.Writer{&archiveStdout, &boardStdout, &doctorStdout, &exportStdout, &initStdout,
		&diffStdout, &mirrorStdout, &newStdout, &pushStdout, &statusStdout, &registryStdout, &syncStdout, &useStdout}
	stderrs := []*io.Writer{&archiveStderr, &boardStderr, &doctorStderr, &exportStderr, &initStderr,
		&diffStderr, &mirrorStderr, &newStderr, &pushStderr, &statusStderr, &registryStderr, &syncStderr, &useStderr}
	var saved []io.Writer
	for _, p := range append(append([]*io.Writer{}, stdouts...), stderrs...) {
		saved = append(saved, *p)
	}
	for _, p := range stdouts {
		*p = out
	}
	for _, p := range stderrs {
		*p = errb
	}
	t.Cleanup(func() {
		for i, p := range append(append([]*io.Writer{}, stdouts...), stderrs...) {
			*p = saved[i]
		}
	})
	return out, errb
}

// writeTwoProjectHome writes settings with projects beta and alpha (alpha is
// current) and returns the home directory.
func writeTwoProjectHome(t *testing.T) (home, alphaDir string) {
	t.Helper()
	home = t.TempDir()
	alphaDir = filepath.Join(home, "alpha")
	betaDir := filepath.Join(home, "beta")
	for _, d := range []string{alphaDir, betaDir, filepath.Join(home, ".jirafs")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	toml := `version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "basic"

[projects.beta]
key = "BETA"
instance = "work"
mirror_dir = "` + betaDir + `"
local_dirs = ["` + betaDir + `"]

[projects.alpha]
key = "ALPHA"
instance = "work"
mirror_dir = "` + alphaDir + `"
local_dirs = ["` + alphaDir + `"]

[state]
current_project = "alpha"
`
	if err := os.WriteFile(filepath.Join(home, ".jirafs", "settings.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	return home, alphaDir
}

// TestPrimaryOutputGoesToStdout runs one happy-path invocation per command
// and checks the primary result lands on stdout while stderr stays free of it.
func TestPrimaryOutputGoesToStdout(t *testing.T) {
	_, alphaDir := writeTwoProjectHome(t)
	if err := os.WriteFile(filepath.Join(alphaDir, "mirror.yml"), []byte(mirrorYAMLContent("ALPHA")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alphaDir, registry.UsersFile), []byte(registryEntryContent), 0o644); err != nil {
		t.Fatal(err)
	}
	chdirTemp(t)

	newMirror := filepath.Join(t.TempDir(), "gamma")
	cases := []struct {
		name string
		run  func() int
		out  string // must appear on stdout
		errs string // must NOT appear on stderr ("" skips)
	}{
		{"init", func() int {
			return RunInit([]string{"gamma", "--key", "GAM", "--instance", "work", "--base-url", "https://jira.example.com", "--mirror-dir", newMirror})
		}, filepath.Join(newMirror, "mirror.yml"), "mirror.yml"},
		{"use list", func() int { return RunUse(nil) }, "- beta", "- beta"},
		{"use set", func() int { return RunUse([]string{"beta"}) }, `current project set to "beta"`, `current project set`},
		{"status", func() int { return RunStatus([]string{"--project", "alpha"}) }, "project: alpha", "project: alpha"},
		{"status porcelain", func() int { return RunStatus([]string{"--porcelain", "--project", "alpha"}) }, "", ""},
		{"doctor", func() int { return RunDoctor([]string{"--project", "alpha"}) }, "project:  alpha", "project:  alpha"},
		{"board", func() int { return RunBoard([]string{"--project", "alpha"}) }, "jirafs board", "jirafs board:"},
		{"diff", func() int { return RunDiff([]string{"--project", "alpha"}) }, "", ""},
		{"registry show", func() int { return RunRegistry([]string{"show", "users", "--project", "alpha"}) }, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, errb := swapStreams(t)
			if code := tc.run(); code != 0 {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, out.String(), errb.String())
			}
			if tc.out != "" && !strings.Contains(out.String(), tc.out) {
				t.Errorf("stdout = %q, want it to contain %q", out.String(), tc.out)
			}
			if tc.errs != "" && strings.Contains(errb.String(), tc.errs) {
				t.Errorf("stderr = %q, must not contain primary result %q", errb.String(), tc.errs)
			}
		})
	}
}

// TestStatusAndDoctorProjectCwd verifies that --project and --cwd select the
// project through the shared flag helper.
func TestStatusAndDoctorProjectCwd(t *testing.T) {
	_, alphaDir := writeTwoProjectHome(t)
	chdirTemp(t)

	for _, tc := range []struct {
		name string
		run  func(args []string) int
		want string
	}{
		{"status", RunStatus, "project: beta"},
		{"doctor", RunDoctor, "project:  beta"},
	} {
		t.Run(tc.name+" --project", func(t *testing.T) {
			out, errb := swapStreams(t)
			if code := tc.run([]string{"--project", "beta"}); code != 0 {
				t.Fatalf("exit = %d, stderr = %q", code, errb.String())
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("stdout = %q, want %q", out.String(), tc.want)
			}
		})
		t.Run(tc.name+" --cwd", func(t *testing.T) {
			out, errb := swapStreams(t)
			if code := tc.run([]string{"--cwd", alphaDir}); code != 0 {
				t.Fatalf("exit = %d, stderr = %q", code, errb.String())
			}
			if !strings.Contains(out.String(), strings.Replace(tc.want, "beta", "alpha", 1)) {
				t.Fatalf("stdout = %q, want alpha project resolved from --cwd", out.String())
			}
		})
		t.Run(tc.name+" --project and --cwd", func(t *testing.T) {
			out, errb := swapStreams(t)
			if code := tc.run([]string{"--project", "beta", "--cwd", alphaDir}); code != 0 {
				t.Fatalf("exit = %d, stderr = %q", code, errb.String())
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("stdout = %q, want explicit --project to win", out.String())
			}
		})
	}
}
