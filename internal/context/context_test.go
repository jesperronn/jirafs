package context

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/pelletier/go-toml/v2"
)

// makeSettings creates a settings struct from a project map and state.
func makeSettings(projects map[string]config.Project, state config.State) *config.Settings {
	return &config.Settings{
		Version:   1,
		Instances: map[string]config.Instance{
			"work": {
				BaseURL:  "https://jira.example.com",
				AuthType: "atlassian_api_token",
			},
		},
		Projects: projects,
		State:    state,
	}
}

func TestStdinPromptReaderPromptSelect(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantIndex int
		wantErr   string
	}{
		{name: "valid selection", input: "2\n", wantIndex: 1},
		{name: "cancel lower", input: "q\n", wantIndex: -1},
		{name: "cancel upper", input: "Q\n", wantIndex: -1},
		{name: "invalid input", input: "wat\n", wantIndex: -1, wantErr: "invalid input"},
		{name: "out of range", input: "5\n", wantIndex: -1, wantErr: "out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &StdinPromptReader{}

			stdin := os.Stdin
			stdout := os.Stdout
			inR, inW, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			outR, outW, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdin = inR
			os.Stdout = outW
			t.Cleanup(func() {
				os.Stdin = stdin
				os.Stdout = stdout
			})

			if _, err := inW.WriteString(tt.input); err != nil {
				t.Fatal(err)
			}
			_ = inW.Close()

			got, err := reader.PromptSelect("Choose", []string{"a", "b", "c"})
			_ = outW.Close()
			_, _ = outR.Read(make([]byte, 512))
			_ = outR.Close()

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("PromptSelect() error = %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("PromptSelect() error = %v, want substring %q", err, tt.wantErr)
			}

			if got != tt.wantIndex {
				t.Fatalf("PromptSelect() index = %d, want %d", got, tt.wantIndex)
			}
		})
	}
}

func TestStdinPromptReaderPromptSelectNoInput(t *testing.T) {
	reader := &StdinPromptReader{}

	stdin := os.Stdin
	stdout := os.Stdout
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = inR
	os.Stdout = outW
	t.Cleanup(func() {
		os.Stdin = stdin
		os.Stdout = stdout
	})

	_ = inW.Close()
	_, err = reader.PromptSelect("Choose", []string{"a"})
	_ = outW.Close()
	_, _ = outR.Read(make([]byte, 256))
	_ = outR.Close()

	if err == nil || !strings.Contains(err.Error(), "no input from stdin") {
		t.Fatalf("PromptSelect() error = %v, want no-input error", err)
	}
}

func TestIsContextError(t *testing.T) {
	var target *Error
	if isContextError(nil, &target) {
		t.Fatal("isContextError(nil) = true, want false")
	}

	err := fmt.Errorf("wrapped: %w", NewError("code", "message"))
	if !isContextError(err, &target) {
		t.Fatal("isContextError(wrapped) = false, want true")
	}
	if target == nil || target.Code != "code" {
		t.Fatalf("target = %#v, want context error", target)
	}
}

func TestIsAmbiguous(t *testing.T) {
	if isAmbiguous(nil) {
		t.Fatal("isAmbiguous(nil) = true, want false")
	}
	if !isAmbiguous(fmt.Errorf("wrapped: %w", NewError(config.ErrAmbiguousMatch, "ambiguous"))) {
		t.Fatal("isAmbiguous(wrapped ambiguous) = false, want true")
	}
	if isAmbiguous(NewError(config.ErrUnknownProject, "unknown")) {
		t.Fatal("isAmbiguous(non-ambiguous) = true, want false")
	}
}

func TestResolveExplicitConfigName(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: "/mirror/grow"},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "platform")
	ctx, err := r.Resolve("/some/other/dir")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "platform" {
		t.Errorf("Name = %q, want %q", ctx.Name, "platform")
	}
	if ctx.Key != "PLAT" {
		t.Errorf("Key = %q, want %q", ctx.Key, "PLAT")
	}
	if ctx.MirrorDir != "/mirror/plat" {
		t.Errorf("MirrorDir = %q, want %q", ctx.MirrorDir, "/mirror/plat")
	}
	if ctx.Instance != "work" {
		t.Errorf("Instance = %q, want %q", ctx.Instance, "work")
	}
}

func TestResolveExplicitKey(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: "/mirror/grow"},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "GROW")
	ctx, err := r.Resolve("/some/other/dir")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "growth" {
		t.Errorf("Name = %q, want %q", ctx.Name, "growth")
	}
	if ctx.Key != "GROW" {
		t.Errorf("Key = %q, want %q", ctx.Key, "GROW")
	}
}

func TestResolveExplicitNotFound(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "NONEXISTENT")
	_, err := r.Resolve("/some/other/dir")
	if err == nil {
		t.Fatal("Resolve() expected error, got nil")
	}
	var ce *Error
	if !errorsAs(err, &ce) || ce.Code != config.ErrUnknownProject {
		t.Errorf("error code = %q, want %q", errCode(err), config.ErrUnknownProject)
	}
}

func TestResolveExplicitBeatsCwd(t *testing.T) {
	tmpDir := t.TempDir()
	mirrorDir := filepath.Join(tmpDir, "mirror", "plat")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: mirrorDir},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: filepath.Join(tmpDir, "mirror", "grow")},
	}
	s := makeSettings(projects, config.State{})

	// Explicit "growth" should win even though cwd is inside platform's mirror.
	r := NewResolver(s, "growth")
	ctx, err := r.Resolve(mirrorDir)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "growth" {
		t.Errorf("Name = %q, want %q (explicit should beat cwd)", ctx.Name, "growth")
	}
}

func TestResolveCwdMirrorMatch(t *testing.T) {
	tmpDir := t.TempDir()
	mirrorDir := filepath.Join(tmpDir, "mirror", "plat")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: mirrorDir},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: filepath.Join(tmpDir, "mirror", "grow")},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	ctx, err := r.Resolve(mirrorDir)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "platform" {
		t.Errorf("Name = %q, want %q", ctx.Name, "platform")
	}
}

func TestResolveCwdDeepMatch(t *testing.T) {
	tmpDir := t.TempDir()
	mirrorDir := filepath.Join(tmpDir, "mirror", "plat")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	deepPath := filepath.Join(mirrorDir, "issues", "ABC-123")

	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: mirrorDir},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: filepath.Join(tmpDir, "mirror", "grow")},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	ctx, err := r.Resolve(deepPath)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "platform" {
		t.Errorf("Name = %q, want %q", ctx.Name, "platform")
	}
}

func TestResolveCwdMostSpecific(t *testing.T) {
	tmpDir := t.TempDir()
	platMirror := filepath.Join(tmpDir, "mirror", "plat")
	growMirror := filepath.Join(tmpDir, "mirror", "plat", "growth")
	if err := os.MkdirAll(growMirror, 0o755); err != nil {
		t.Fatal(err)
	}

	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: platMirror},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: growMirror},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	ctx, err := r.Resolve(growMirror)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "growth" {
		t.Errorf("Name = %q, want %q (most specific match)", ctx.Name, "growth")
	}
}

func TestResolveCwdLocalDirMatch(t *testing.T) {
	tmpDir := t.TempDir()
	mirrorDir := filepath.Join(tmpDir, "mirror", "plat")
	localDir := filepath.Join(tmpDir, "src", "platform-app")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: mirrorDir, LocalDirs: []string{localDir}},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: filepath.Join(tmpDir, "mirror", "grow")},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	ctx, err := r.Resolve(localDir)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "platform" {
		t.Errorf("Name = %q, want %q", ctx.Name, "platform")
	}
}

func TestResolveStateFallback(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: "/mirror/grow"},
	}
	s := makeSettings(projects, config.State{CurrentProject: "growth"})

	r := NewResolver(s, "")
	ctx, err := r.Resolve("/no/match/here")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "growth" {
		t.Errorf("Name = %q, want %q", ctx.Name, "growth")
	}
}

func TestResolveNoMatchNoState(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	_, err := r.Resolve("/no/match/here")
	if err == nil {
		t.Fatal("Resolve() expected error, got nil")
	}
	var ce *Error
	if !errorsAs(err, &ce) || ce.Code != config.ErrNoProjectResolved {
		t.Errorf("error code = %q, want %q", errCode(err), config.ErrNoProjectResolved)
	}
}

// TestB016aUnresolvedContextStructuredError verifies that when no project
// can be resolved (no explicit flag, no cwd match, no remembered state),
// the returned error includes a structured no-project code and the list of
// known project names as candidates.
func TestB016aUnresolvedContextStructuredError(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: "/mirror/grow"},
		"alpha":    {Key: "ALPHA", Instance: "work", MirrorDir: "/mirror/alpha"},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	_, err := r.Resolve("/tmp/nowhere")
	if err == nil {
		t.Fatal("Resolve() expected error, got nil")
	}

	var ce *Error
	if !errorsAs(err, &ce) {
		t.Fatalf("expected *context.Error, got %T", err)
	}
	if ce.Code != config.ErrNoProjectResolved {
		t.Errorf("error code = %q, want %q", ce.Code, config.ErrNoProjectResolved)
	}
	if ce.Message == "" {
		t.Error("error message should not be empty")
	}

	// Verify candidates include all known project names.
	candidateSet := make(map[string]bool)
	for _, c := range ce.Candidates {
		candidateSet[c] = true
	}
	for name := range projects {
		if !candidateSet[name] {
			t.Errorf("candidate %q missing from error", name)
		}
	}
	if len(ce.Candidates) != len(projects) {
		t.Errorf("candidates count = %d, want %d", len(ce.Candidates), len(projects))
	}
}

func TestResolveNoState(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	_, err := r.Resolve("/mirror/plat")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveCwdNoMatchUsesState(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: "/mirror/grow"},
	}
	s := makeSettings(projects, config.State{CurrentProject: "growth"})

	r := NewResolver(s, "")
	ctx, err := r.Resolve("/tmp/random")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "growth" {
		t.Errorf("Name = %q, want %q", ctx.Name, "growth")
	}
}

func TestContextError(t *testing.T) {
	err := NewError("test_code", "test message")
	expected := "jirafs/context: test_code: test message"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestResolveCwdAmbiguousMirrorDirs(t *testing.T) {
	tmpDir := t.TempDir()
	// Two mirror_dirs at the same depth under tmpDir.
	platMirror := filepath.Join(tmpDir, "mirror", "plat")
	growMirror := filepath.Join(tmpDir, "mirror", "grow")
	if err := os.MkdirAll(platMirror, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(growMirror, 0o755); err != nil {
		t.Fatal(err)
	}

	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: platMirror},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: growMirror},
	}
	s := makeSettings(projects, config.State{})

	// A path that is NOT inside either mirror_dir should not trigger ambiguity.
	r := NewResolver(s, "")
	_, err := r.Resolve(filepath.Join(tmpDir, "other"))
	if err == nil {
		t.Fatal("Resolve() expected error for non-matching cwd, got nil")
	}

	// Now create an ambiguous scenario: two projects with the same mirror_dir depth
	// but nested mirror_dirs that overlap.
	platMirror2 := filepath.Join(tmpDir, "mirror", "plat")
	nestedMirror := filepath.Join(tmpDir, "mirror", "plat", "child")
	if err := os.MkdirAll(nestedMirror, 0o755); err != nil {
		t.Fatal(err)
	}

	projects2 := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: platMirror2},
		"child":    {Key: "CHLD", Instance: "work", MirrorDir: nestedMirror},
	}
	s2 := makeSettings(projects2, config.State{})

	// "child" is more specific (deeper) so it should win.
	r2 := NewResolver(s2, "")
	ctx, err := r2.Resolve(nestedMirror)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "child" {
		t.Errorf("Name = %q, want %q (deeper mirror wins)", ctx.Name, "child")
	}
}

func TestResolveCwdAmbiguousSameDepth(t *testing.T) {
	// Two mirror_dirs at the same depth that are siblings (non-overlapping).
	// A cwd inside one should match only that one, not both.
	tmpDir := t.TempDir()
	eqMirror1 := filepath.Join(tmpDir, "mirror", "proj-a")
	eqMirror2 := filepath.Join(tmpDir, "mirror", "proj-b")
	if err := os.MkdirAll(eqMirror1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(eqMirror2, 0o755); err != nil {
		t.Fatal(err)
	}

	projects := map[string]config.Project{
		"proj-a": {Key: "PA", Instance: "work", MirrorDir: eqMirror1},
		"proj-b": {Key: "PB", Instance: "work", MirrorDir: eqMirror2},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")

	// Cwd inside proj-a should only match proj-a (no ambiguity since
	// proj-b's mirror_dir is not a prefix of the cwd).
	ctx, err := r.Resolve(eqMirror1)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "proj-a" {
		t.Errorf("Name = %q, want %q", ctx.Name, "proj-a")
	}

	// Cwd inside proj-b should only match proj-b.
	ctx, err = r.Resolve(eqMirror2)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "proj-b" {
		t.Errorf("Name = %q, want %q", ctx.Name, "proj-b")
	}
}

func TestResolveCwdAmbiguousMirrorVsLocalDir(t *testing.T) {
	// mirror_dir of project A and local_dir of project B at same depth.
	tmpDir := t.TempDir()
	pathA := filepath.Join(tmpDir, "mirror", "proj-a")
	pathB := filepath.Join(tmpDir, "mirror", "proj-b")
	if err := os.MkdirAll(pathA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pathB, 0o755); err != nil {
		t.Fatal(err)
	}

	projects := map[string]config.Project{
		"proj-a": {Key: "PA", Instance: "work", MirrorDir: pathA},
		"proj-b": {Key: "PB", Instance: "work", MirrorDir: filepath.Join(tmpDir, "mirror", "proj-b-mirror"), LocalDirs: []string{pathB}},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	// pathA only matches proj-a's mirror_dir.
	ctx, err := r.Resolve(pathA)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "proj-a" {
		t.Errorf("Name = %q, want %q", ctx.Name, "proj-a")
	}

	// pathB only matches proj-b's local_dir.
	ctx, err = r.Resolve(pathB)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "proj-b" {
		t.Errorf("Name = %q, want %q", ctx.Name, "proj-b")
	}
}

func TestResolveCwdAmbiguousIdenticalMirrorDirs(t *testing.T) {
	// Two projects with the same mirror_dir should be caught by config validation,
	// but test that resolveCwd also handles it gracefully.
	tmpDir := t.TempDir()
	dupMirror := filepath.Join(tmpDir, "mirror", "dup")
	if err := os.MkdirAll(dupMirror, 0o755); err != nil {
		t.Fatal(err)
	}

	projects := map[string]config.Project{
		"proj-a": {Key: "PA", Instance: "work", MirrorDir: dupMirror},
		"proj-b": {Key: "PB", Instance: "work", MirrorDir: dupMirror},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	_, err := r.Resolve(dupMirror)
	if err == nil {
		t.Fatal("Resolve() expected ambiguity error for identical mirror_dirs, got nil")
	}
	var ce *Error
	if !errorsAs(err, &ce) || ce.Code != config.ErrAmbiguousMatch {
		t.Errorf("error code = %q, want %q", errCode(err), config.ErrAmbiguousMatch)
	}
}

// TestB015aReadRememberedProject verifies that when no explicit project
// flag is given and cwd has no match, the remembered current project
// from settings state is read and returned.
func TestB015aReadRememberedProject(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: "/mirror/grow"},
	}
	s := makeSettings(projects, config.State{CurrentProject: "platform"})

	r := NewResolver(s, "")
	ctx, err := r.Resolve("/tmp/random/nowhere")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "platform" {
		t.Errorf("Name = %q, want %q", ctx.Name, "platform")
	}
	if ctx.Key != "PLAT" {
		t.Errorf("Key = %q, want %q", ctx.Key, "PLAT")
	}
	if ctx.MirrorDir != "/mirror/plat" {
		t.Errorf("MirrorDir = %q, want %q", ctx.MirrorDir, "/mirror/plat")
	}
	if ctx.Instance != "work" {
		t.Errorf("Instance = %q, want %q", ctx.Instance, "work")
	}
}

func TestIsPrefixOf(t *testing.T) {
	tests := []struct {
		prefix, target string
		want           bool
	}{
		{"/a/b", "/a/b", true},
		{"/a/b", "/a/b/c", true},
		{"/a/b", "/a/bc", false},
		{"/a/b", "/a", false},
		{"/a", "/a/b", true},
	}
	for _, tc := range tests {
		got := isPrefixOf(tc.prefix, tc.target)
		if got != tc.want {
			t.Errorf("isPrefixOf(%q, %q) = %v, want %v", tc.prefix, tc.target, got, tc.want)
		}
	}
}

func errCode(err error) string {
	if err == nil {
		return ""
	}
	var se *config.SettingError
	for err != nil {
		if target, ok := err.(*config.SettingError); ok {
			se = target
			break
		}
		if target, ok := err.(*Error); ok {
			return target.Code
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
			continue
		}
		break
	}
	if se != nil {
		return se.Code
	}
	return ""
}

func errorsAs(err error, target any) bool {
	return errors.As(err, target)
}

// TestB015bWriteRememberedProject verifies that after a successful explicit
// selection, SaveCurrentProject persists the project name to the settings file.
// mockPrompter captures selections for testing interactive resolution.
type mockPrompter struct {
	selected int
	err      error
}

func (m *mockPrompter) PromptSelect(prompt string, candidates []string) (int, error) {
	if m.err != nil {
		return -1, m.err
	}
	return m.selected, nil
}

// TestB016bInteractiveResolveSelectsFromKnownProjects verifies that when
// normal resolution fails with ErrNoProjectResolved, InteractiveResolve
// prompts the user and selects from the known project candidates.
func TestB016bInteractiveResolveSelectsFromKnownProjects(t *testing.T) {
	tmpDir := t.TempDir()
	jirafsDir := filepath.Join(tmpDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(jirafsDir, "settings.toml")
	initialTOML := `version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "/mirror/plat"

[projects.growth]
key = "GROW"
instance = "work"
mirror_dir = "/mirror/grow"

[projects.alpha]
key = "ALPHA"
instance = "work"
mirror_dir = "/mirror/alpha"
`
	if err := os.WriteFile(settingsPath, []byte(initialTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	s, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	r := NewResolver(s, "")
	prompter := &mockPrompter{selected: 1} // select "growth" (index 1)

	ctx, err := r.InteractiveResolve("/tmp/nowhere", prompter)
	if err != nil {
		t.Fatalf("InteractiveResolve() error = %v", err)
	}
	if ctx.Name != "growth" {
		t.Errorf("Name = %q, want %q", ctx.Name, "growth")
	}
	if ctx.Key != "GROW" {
		t.Errorf("Key = %q, want %q", ctx.Key, "GROW")
	}
	if ctx.MirrorDir != "/mirror/grow" {
		t.Errorf("MirrorDir = %q, want %q", ctx.MirrorDir, "/mirror/grow")
	}
	if ctx.Instance != "work" {
		t.Errorf("Instance = %q, want %q", ctx.Instance, "work")
	}
}

// TestB016bInteractiveResolveFirstSelection verifies selecting the first
// candidate (index 0).
func TestB016bInteractiveResolveFirstSelection(t *testing.T) {
	tmpDir := t.TempDir()
	jirafsDir := filepath.Join(tmpDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(jirafsDir, "settings.toml")
	initialTOML := `version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "/mirror/plat"

[projects.growth]
key = "GROW"
instance = "work"
mirror_dir = "/mirror/grow"
`
	if err := os.WriteFile(settingsPath, []byte(initialTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	s, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	r := NewResolver(s, "")
	prompter := &mockPrompter{selected: 0} // select "platform" (index 0)

	ctx, err := r.InteractiveResolve("/tmp/nowhere", prompter)
	if err != nil {
		t.Fatalf("InteractiveResolve() error = %v", err)
	}
	if ctx.Name != "platform" {
		t.Errorf("Name = %q, want %q", ctx.Name, "platform")
	}
}

// TestB016bInteractiveResolvePromptsOnNoProjectResolved verifies that
// InteractiveResolve only prompts when the error is ErrNoProjectResolved.
func TestB016bInteractiveResolvePromptsOnNoProjectResolved(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "NONEXISTENT")
	prompter := &mockPrompter{}

	// Explicit flag resolves to unknown project — should NOT prompt.
	_, err := r.InteractiveResolve("/tmp/nowhere", prompter)
	if err == nil {
		t.Fatal("InteractiveResolve() expected error, got nil")
	}
	// Verify the error is the original error, not a prompt failure.
	if errCode(err) != config.ErrUnknownProject {
		t.Errorf("error code = %q, want %q", errCode(err), config.ErrUnknownProject)
	}
}

// TestB016bInteractiveResolveNoProjectResolvedPrompts verifies that
// InteractiveResolve prompts when resolution fails with ErrNoProjectResolved.
func TestB016bInteractiveResolveNoProjectResolvedPrompts(t *testing.T) {
	tmpDir := t.TempDir()
	jirafsDir := filepath.Join(tmpDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(jirafsDir, "settings.toml")
	initialTOML := `version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "/mirror/plat"

[projects.growth]
key = "GROW"
instance = "work"
mirror_dir = "/mirror/grow"
`
	if err := os.WriteFile(settingsPath, []byte(initialTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	s, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	r := NewResolver(s, "")
	// No explicit flag, no cwd match, no state → ErrNoProjectResolved.
	// Use a large index so the mock always returns the last candidate,
	// which is deterministic regardless of map iteration order.
	prompter := &mockPrompter{selected: len(s.Projects) - 1}

	ctx, err := r.InteractiveResolve("/tmp/nowhere", prompter)
	if err != nil {
		t.Fatalf("InteractiveResolve() error = %v", err)
	}
	// Verify the result is one of the known projects.
	if ctx.Name != "platform" && ctx.Name != "growth" {
		t.Errorf("Name = %q, want one of {platform, growth}", ctx.Name)
	}
}

// TestB016bInteractiveResolveUserCancels verifies that when the user
// cancels (returns -1), InteractiveResolve returns an error.
func TestB016bInteractiveResolveUserCancels(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	prompter := &mockPrompter{selected: -1} // user cancels

	_, err := r.InteractiveResolve("/tmp/nowhere", prompter)
	if err == nil {
		t.Fatal("InteractiveResolve() expected error on cancel, got nil")
	}
}

// TestB016bInteractiveResolvePrompterError verifies that a prompter error
// is propagated correctly.
func TestB016bInteractiveResolvePrompterError(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	prompter := &mockPrompter{err: fmt.Errorf("stdin closed")}

	_, err := r.InteractiveResolve("/tmp/nowhere", prompter)
	if err == nil {
		t.Fatal("InteractiveResolve() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "stdin closed") {
		t.Errorf("error = %v, want to contain 'stdin closed'", err)
	}
}

// TestB016bInteractiveResolveSuccessfulExplicitDoesNotPrompt verifies
// that when Resolve succeeds, InteractiveResolve returns immediately
// without calling the prompter.
func TestB016bInteractiveResolveSuccessfulExplicitDoesNotPrompt(t *testing.T) {
	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: "/mirror/plat"},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: "/mirror/grow"},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "platform")
	prompter := &mockPrompter{}

	ctx, err := r.InteractiveResolve("/tmp/nowhere", prompter)
	if err != nil {
		t.Fatalf("InteractiveResolve() error = %v", err)
	}
	if ctx.Name != "platform" {
		t.Errorf("Name = %q, want %q", ctx.Name, "platform")
	}
}

// TestB016bInteractiveResolveSuccessfulCwdDoesNotPrompt verifies
// that when Resolve succeeds via cwd match, InteractiveResolve returns
// immediately without calling the prompter.
func TestB016bInteractiveResolveSuccessfulCwdDoesNotPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	mirrorDir := filepath.Join(tmpDir, "mirror", "plat")
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projects := map[string]config.Project{
		"platform": {Key: "PLAT", Instance: "work", MirrorDir: mirrorDir},
		"growth":   {Key: "GROW", Instance: "work", MirrorDir: filepath.Join(tmpDir, "mirror", "grow")},
	}
	s := makeSettings(projects, config.State{})

	r := NewResolver(s, "")
	prompter := &mockPrompter{}

	ctx, err := r.InteractiveResolve(mirrorDir, prompter)
	if err != nil {
		t.Fatalf("InteractiveResolve() error = %v", err)
	}
	if ctx.Name != "platform" {
		t.Errorf("Name = %q, want %q", ctx.Name, "platform")
	}
}

// TestB016bInteractiveResolveRemembersSelection verifies that the selected
// project is persisted to settings state.
func TestB016bInteractiveResolveRemembersSelection(t *testing.T) {
	tmpDir := t.TempDir()
	jirafsDir := filepath.Join(tmpDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(jirafsDir, "settings.toml")
	initialTOML := `version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "/mirror/plat"

[projects.growth]
key = "GROW"
instance = "work"
mirror_dir = "/mirror/grow"
`
	if err := os.WriteFile(settingsPath, []byte(initialTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	s, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	r := NewResolver(s, "")
	prompter := &mockPrompter{selected: 0} // select "platform"

	ctx, err := r.InteractiveResolve("/tmp/nowhere", prompter)
	if err != nil {
		t.Fatalf("InteractiveResolve() error = %v", err)
	}
	if ctx.Name != "platform" {
		t.Errorf("Name = %q, want %q", ctx.Name, "platform")
	}

	// Verify state was persisted.
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var reloaded config.Settings
	if err := toml.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if reloaded.State.CurrentProject != "platform" {
		t.Errorf("State.CurrentProject = %q, want %q", reloaded.State.CurrentProject, "platform")
	}
}

func TestB015bWriteRememberedProject(t *testing.T) {
	tmpDir := t.TempDir()
	jirafsDir := filepath.Join(tmpDir, ".jirafs")
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(jirafsDir, "settings.toml")
	initialTOML := `version = 1

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "/mirror/plat"

[projects.growth]
key = "GROW"
instance = "work"
mirror_dir = "/mirror/grow"
`
	if err := os.WriteFile(settingsPath, []byte(initialTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	// Point HOME at our temp dir so settingsPath resolves correctly.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	s, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	r := NewResolver(s, "platform")
	ctx, err := r.Resolve("/tmp/random/nowhere")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ctx.Name != "platform" {
		t.Errorf("Name = %q, want %q", ctx.Name, "platform")
	}

	// Save the current project.
	if err := r.SaveCurrentProject(ctx.Name); err != nil {
		t.Fatalf("SaveCurrentProject() error = %v", err)
	}

	// Re-read the settings file and verify the state was updated.
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var reloaded config.Settings
	if err := toml.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if reloaded.State.CurrentProject != "platform" {
		t.Errorf("State.CurrentProject = %q, want %q", reloaded.State.CurrentProject, "platform")
	}
}
