package cli

import (
	"strings"
	"testing"
)

// TestRunPull_NoArgPullsEveryConfiguredScope verifies that a bare pull
// searches each scope in mirror.yml and nothing else.
func TestRunPull_NoArgPullsEveryConfiguredScope(t *testing.T) {
	e := newIncrEnv(t, "  - name: team\n    type: jql\n    target: project = TEST\n")
	if code := RunPull([]string{"--project", "test"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, e.stderr.String())
	}
	seen := map[string]bool{}
	for _, jql := range e.rec.jqls {
		seen[jql] = true
	}
	if len(seen) != 2 || !seen["assignee = currentUser()"] || !seen["project = TEST"] {
		t.Fatalf("searches = %q, want exactly the two configured scopes", e.rec.jqls)
	}
	for _, name := range []string{`scope "my-issues"`, `scope "team"`} {
		if !strings.Contains(e.stdout.String(), name) {
			t.Errorf("stdout missing %s:\n%s", name, e.stdout.String())
		}
	}
}

// TestRunPull_NoArgDryRunPrintsOneRealCommand verifies that a bare dry run
// lists every scope and ends with the single real command.
func TestRunPull_NoArgDryRunPrintsOneRealCommand(t *testing.T) {
	e := newIncrEnv(t, "  - name: team\n    type: jql\n    target: project = TEST\n")
	if code := RunPull([]string{"--project", "test", "--dry-run", "--full"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, e.stderr.String())
	}
	out := e.stdout.String()
	if strings.Count(out, "would write") != 2 {
		t.Errorf("want a dry-run listing per scope:\n%s", out)
	}
	if strings.Count(out, "to run it for real:") != 1 || !strings.Contains(out, "to run it for real: jirafs pull --full") {
		t.Errorf("want exactly one real command:\n%s", out)
	}
}

// TestRunPull_NoArgReturnsFailureOfAnyScope verifies that one failing scope
// makes the whole pull exit non-zero while the others still run.
func TestRunPull_NoArgReturnsFailureOfAnyScope(t *testing.T) {
	e := newIncrEnv(t, "  - name: bad\n    type: jql\n    target: ''\n")
	if code := RunPull([]string{"--project", "test"}); code == 0 {
		t.Fatalf("exit = 0, want non-zero; stdout=%q stderr=%q", e.stdout.String(), e.stderr.String())
	}
	if !strings.Contains(e.stdout.String(), `scope "my-issues"`) {
		t.Errorf("healthy scope was skipped:\n%s", e.stdout.String())
	}
}

// TestRunPull_KeyAndScopeNameConflict verifies that a scope named like an
// issue key is an error telling the user to rename it.
func TestRunPull_KeyAndScopeNameConflict(t *testing.T) {
	e := newIncrEnv(t, "  - name: TEST-1\n    type: jql\n    target: project = TEST\n")
	if code := RunPull([]string{"--project", "test", "TEST-1"}); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	for _, want := range []string{"both an issue key and a scope name", "rename the scope"} {
		if !strings.Contains(e.stderr.String(), want) {
			t.Errorf("stderr missing %q: %q", want, e.stderr.String())
		}
	}
	if len(e.rec.jqls) != 0 {
		t.Errorf("searched despite conflict: %q", e.rec.jqls)
	}
}

func TestIsIssueArg(t *testing.T) {
	for arg, want := range map[string]bool{
		"PLAT-123": true, "A1-9": true, "https://jira.example.com/browse/PLAT-1": true,
		"my-issues": false, "plat-1": false, "PLAT-": false, "P-1": false, "": false,
	} {
		if got := isIssueArg(arg); got != want {
			t.Errorf("isIssueArg(%q) = %v, want %v", arg, got, want)
		}
	}
}
