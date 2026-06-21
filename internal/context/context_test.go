package context

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
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
