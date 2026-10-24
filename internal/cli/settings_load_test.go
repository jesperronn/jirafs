package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingSettingsPrintsFirstRunHint(t *testing.T) {
	cases := []struct {
		name   string
		stderr *io.Writer
		run    func() int
	}{
		{"sync", &syncStderr, func() int { return RunSync([]string{"PLAT-1"}) }},
		{"export", &exportStderr, func() int { return RunExport([]string{"issue", "PLAT-1"}) }},
		{"diff", &diffStderr, func() int { return RunDiff([]string{"PLAT-1"}) }},
		{"mirror", &mirrorStderr, func() int { return RunMirror([]string{"refresh", "my-issues"}) }},
		{"board", &boardStderr, func() int { return RunBoard(nil) }},
		{"status", &statusStderr, func() int { return RunStatus(nil) }},
		{"doctor", &doctorStderr, func() int { return RunDoctor(nil) }},
		{"registry show", &registryStderr, func() int { return RunRegistry([]string{"show", "users"}) }},
		{"registry init", &registryStderr, func() int { return RunRegistry([]string{"init", "users"}) }},
		{"archive issue", &archiveStderr, func() int { return RunArchive([]string{"issue", "PLAT-1"}) }},
		{"archive list", &archiveStderr, func() int { return RunArchive([]string{"list"}) }},
		{"use clear", &useStderr, func() int { return RunUse([]string{"--clear"}) }},
		{"use show", &useStderr, func() int { return RunUse(nil) }},
		{"use project", &useStderr, func() int { return RunUse([]string{"--project", "PLAT"}) }},
		{"new", &newStderr, func() int { return RunNew([]string{"--summary", "x"}) }},
		{"push", &pushStderr, func() int { return RunPush([]string{writeFilledDraft(t)}) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			var buf bytes.Buffer
			old := *tc.stderr
			*tc.stderr = &buf
			t.Cleanup(func() { *tc.stderr = old })

			if code := tc.run(); code != 1 {
				t.Fatalf("exit code = %d, want 1; output:\n%s", code, buf.String())
			}
			out := buf.String()
			for _, want := range []string{
				"no settings found (~/.jirafs/settings.toml)",
				"next step: jirafs setup --project",
				"--credential-ref env://",
				"--set-current",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q:\n%s", want, out)
				}
			}
		})
	}
}

// writeFilledDraft writes a valid draft issue and returns its path.
func writeFilledDraft(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "draft.md")
	if err := os.WriteFile(p, []byte(filledStory), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadSettingsOrHintParseErrorNamesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".jirafs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "settings.toml")
	if err := os.WriteFile(path, []byte("version = = broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	s, ok := loadSettingsOrHint(&buf, "sync")
	if ok || s != nil {
		t.Fatalf("loadSettingsOrHint = (%v, %v), want (nil, false)", s, ok)
	}
	out := buf.String()
	if !strings.Contains(out, "jirafs sync: cannot load settings:") {
		t.Errorf("missing error message:\n%s", out)
	}
	if !strings.Contains(out, path) {
		t.Errorf("output does not name %s:\n%s", path, out)
	}
	if strings.Contains(out, "next step:") {
		t.Errorf("parse error must not print first-run hint:\n%s", out)
	}
}
