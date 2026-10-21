package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/codec"
)

const filledStory = "---\ntype: story\nproject: \"project:PROJECT\"\nschema_version: \"1\"\nstate: draft\nsummary: Add dark mode\n---\n\n## Description\n\nUsers want dark mode.\n\n## Acceptance Criteria\n\n- Toggle in settings.\n"

func TestNewPromptPrintsTemplateAndIdea(t *testing.T) {
	out, errBuf := withNewTestIO(t)
	if code := RunNew([]string{"story", "--prompt", "--idea", "dark mode please"}); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errBuf)
	}
	for _, want := range []string{"Output only the completed document", "type: story", "## Acceptance Criteria", "dark mode please"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("prompt missing %q:\n%s", want, out)
		}
	}
}

func TestNewPromptWithoutIdeaAndErrors(t *testing.T) {
	out, errBuf := withNewTestIO(t)
	if code := RunNew([]string{"bug", "--prompt"}); code != 0 || strings.Contains(out.String(), "Idea:") {
		t.Fatalf("code=%d out=%s", code, out)
	}
	if code := RunNew([]string{"--prompt"}); code != 1 || !strings.Contains(errBuf.String(), "jirafs new story --prompt") {
		t.Fatalf("want hint, got code=%d err=%s", code, errBuf)
	}
	tmpl := filepath.Join(t.TempDir(), "t.md")
	_ = os.WriteFile(tmpl, []byte(filledStory), 0o644)
	out.Reset()
	if code := RunNew([]string{"--prompt", "--template", tmpl}); code != 0 || !strings.Contains(out.String(), "Add dark mode") {
		t.Fatalf("custom template prompt failed: %s", out)
	}
	if code := RunNew([]string{"--prompt", "--template", tmpl + ".missing"}); code != 1 {
		t.Fatalf("missing template should fail")
	}
}

func TestNewFromFileAndOverrides(t *testing.T) {
	localDir := setupNewTest(t)
	out, errBuf := withNewTestIO(t)
	src := filepath.Join(t.TempDir(), "in.md")
	_ = os.WriteFile(src, []byte(filledStory), 0o644)
	code := RunNew([]string{"--from", src, "--assignee", "alice", "--cwd", localDir})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errBuf)
	}
	if !strings.HasPrefix(out.String(), "jirafs: created ") {
		t.Fatalf("stdout=%q", out)
	}
	data, _ := os.ReadFile(firstMarkdownFile(t, localDir))
	issue, pe := codec.ParseIssue(string(data))
	if pe != nil {
		t.Fatal(pe)
	}
	if issue.Summary != "Add dark mode" || issue.Assignee == nil || *issue.Assignee != "alice" {
		t.Errorf("issue=%+v", issue)
	}
	if issue.Identity.Project.Value != "TEST" {
		t.Errorf("project=%v, want TEST", issue.Identity.Project)
	}
	if issue.RemoteMetadata.StateFile != "draft" {
		t.Errorf("state=%q", issue.RemoteMetadata.StateFile)
	}
}

func TestNewFromStdinWithFenceAndProject(t *testing.T) {
	localDir := setupNewTest(t)
	_, errBuf := withNewTestIO(t)
	newStdin = strings.NewReader("```markdown\n" + filledStory + "```\n")
	t.Cleanup(func() { newStdin = os.Stdin })
	if code := RunNew([]string{"--from", "-", "--project", "TEST", "--cwd", localDir}); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errBuf)
	}
	data, _ := os.ReadFile(firstMarkdownFile(t, localDir))
	if !strings.Contains(string(data), "Add dark mode") {
		t.Errorf("draft content: %s", data)
	}
}

func TestNewFromValidationErrors(t *testing.T) {
	cases := map[string]struct{ doc, want string }{
		"no summary":  {"---\ntype: story\nstate: draft\n---\n", `field "summary"`},
		"no type":     {"---\nsummary: x\nstate: draft\n---\n", `field "type"`},
		"bad yaml":    {"---\ntype: [\n---\n", "invalid_yaml"},
		"no closing":  {"---\ntype: story\n", "no_closing_delimiter"},
		"bad section": {"---\ntype: story\nsummary: x\n---\n\n## Bogus\n\nx\n", "unknown_section"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			localDir := setupNewTest(t)
			_, errBuf := withNewTestIO(t)
			newStdin = strings.NewReader(c.doc)
			t.Cleanup(func() { newStdin = os.Stdin })
			if code := RunNew([]string{"--from", "-", "--cwd", localDir}); code != 1 {
				t.Fatalf("exit=%d, want 1", code)
			}
			e := errBuf.String()
			if !strings.Contains(e, c.want) || !strings.Contains(e, "jirafs new --from -") {
				t.Errorf("stderr=%q, want %q and retry hint", e, c.want)
			}
		})
	}
}

func TestNewFromFileErrors(t *testing.T) {
	localDir := setupNewTest(t)
	_, errBuf := withNewTestIO(t)
	if code := RunNew([]string{"--from", "/no/such/file.md", "--cwd", localDir}); code != 1 {
		t.Fatalf("missing file should fail")
	}
	bad := filepath.Join(t.TempDir(), "bad.md")
	_ = os.WriteFile(bad, []byte("---\nsummary: x\n---\n"), 0o644)
	errBuf.Reset()
	if code := RunNew([]string{"--from", bad, "--cwd", localDir}); code != 1 || !strings.Contains(errBuf.String(), "jirafs new --from "+bad) {
		t.Fatalf("want file retry hint, got %s", errBuf)
	}
}

func TestStripCodeFencePlain(t *testing.T) {
	if got := stripCodeFence("abc"); got != "abc" {
		t.Errorf("got %q", got)
	}
	if got := stripCodeFence("```\nabc"); got != "abc" {
		t.Errorf("got %q", got)
	}
}
