// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/gitx"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/registry"
)

var (
	initStdout io.Writer = os.Stdout
	initStderr io.Writer = os.Stderr
)

// layoutFile describes one file that init writes into the mirror directory.
type layoutFile struct {
	// name is the file name relative to the mirror directory.
	name string
	// content is the exact bytes written when the file does not yet exist.
	content string
}

// registryEntryContent is the valid-but-empty body written to every registry
// file. It matches what the registry loaders accept (see registry package).
const registryEntryContent = "entries: {}\n"

// RunInit handles the `jirafs init` command, the single way to configure a
// project. It records one Jira instance and one project in
// ~/.jirafs/settings.toml (creating the file if needed), creates the mirror
// directory, and writes mirror.yml plus the six registry files. The project
// name is the first positional argument; --key is the Jira project key. The
// command is idempotent for files: existing files are never overwritten, and
// each is reported as created or skipped.
func RunInit(args []string) int {
	fs := quietFlagSet("init")
	keyFlag := fs.String("key", "", "Jira project key (e.g. PLAT)")
	instanceFlag := fs.String("instance", "", "settings name for the Jira instance")
	baseURLFlag := fs.String("base-url", "", "Jira base URL (http:// or https://)")
	authTypeFlag := fs.String("auth-type", "", "auth type: basic, atlassian_api_token, bearer_token or oauth1 (default: atlassian_api_token)")
	mirrorDirFlag := fs.String("mirror-dir", "", "mirror directory (default: ~/jira/<project>)")
	gitFlag := fs.Bool("git", false, "make the mirror git-backed: set git: true in mirror.yml, run git init if needed, write .gitignore")
	setCurrentFlag := fs.Bool("set-current", false, "also set the remembered current project")
	var credentialRefs []string
	fs.Func("credential-ref", "credential reference (env://..., file://... or op://...); may be repeated", func(v string) error {
		credentialRefs = append(credentialRefs, v)
		return nil
	})
	if wantsHelpOrWord(args) {
		printInitHelp(fs)
		return 0
	}
	positionals, err := parseInterspersed(fs, args)
	if err != nil {
		return flagError(initStderr, "jirafs init", err)
	}
	if len(positionals) > 1 {
		return usageError(initStderr, "jirafs init", "too many positional arguments; expected one project name")
	}
	name := ""
	if len(positionals) == 1 {
		name = positionals[0]
	}
	key, instance, baseURL, authType, mirrorDir := *keyFlag, *instanceFlag, *baseURLFlag, *authTypeFlag, *mirrorDirFlag

	if isInteractiveInput() {
		reader := bufio.NewReader(os.Stdin)
		promptMissingValue(reader, &name, "project name", "settings name for the project")
		promptMissingValue(reader, &key, "--key", "Jira project key (for example PLAT)")
		promptMissingValue(reader, &instance, "--instance", "settings name for the Jira instance")
		promptMissingValue(reader, &baseURL, "--base-url", "Jira base URL")
		promptMissingValue(reader, &authType, "--auth-type", "auth type (basic, atlassian_api_token, bearer_token, oauth1; default atlassian_api_token)")
		promptMissingValue(reader, &mirrorDir, "--mirror-dir", "local mirror directory path (default ~/jira/<project>)")
	}

	defaultsUsed := applyInitDefaults(name, &authType, &mirrorDir)

	if missing := missingInitValues(name, key, instance, baseURL); len(missing) > 0 {
		return usageError(initStderr, "jirafs init", "missing required values: %s (pass them as arguments/flags or run init interactively from a terminal)", strings.Join(missing, ", "))
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return usageError(initStderr, "jirafs init", "--base-url must start with http:// or https://")
	}

	if *gitFlag && !gitx.Available() {
		fmt.Fprintln(initStderr, gitx.NotInstalledHint)
		return 1
	}

	fmt.Fprintf(initStderr, "jirafs init: configuring project %q (key %q) on instance %q at %q using auth %q\n", name, key, instance, baseURL, authType)
	for _, msg := range defaultsUsed {
		fmt.Fprintf(initStderr, "jirafs init: default %s\n", msg)
	}

	expanded, err := expandTilde(mirrorDir)
	if err != nil {
		fmt.Fprintf(initStderr, "jirafs init: cannot expand mirror directory %q: %v\n", mirrorDir, err)
		return 1
	}
	fmt.Fprintf(initStderr, "jirafs init: ensuring mirror directory %q exists\n", expanded)
	if err := os.MkdirAll(expanded, 0o755); err != nil {
		fmt.Fprintf(initStderr, "jirafs init: cannot create mirror directory %q: %v\n", expanded, err)
		return 1
	}

	// Load existing settings, or start fresh when none exist yet.
	s, err := config.Load()
	if err != nil {
		if !isSettingsMissing(err) {
			fmt.Fprintf(initStderr, "jirafs init: cannot load settings: %v\n", err)
			return 1
		}
		s = &config.Settings{
			Version:   1,
			Instances: make(map[string]config.Instance),
			Projects:  make(map[string]config.Project),
		}
	}

	fmt.Fprintln(initStderr, "jirafs init: writing settings")
	if err := s.SetupProject(instance, name, key, baseURL, authType, mirrorDir, credentialRefs); err != nil {
		fmt.Fprintf(initStderr, "jirafs init: %v\n", err)
		return 1
	}

	fmt.Fprintln(initStderr, "jirafs init: ensuring project layout")
	created, skipped, err := ensureLayout(expanded, initLayout(expanded, key))
	if err != nil {
		fmt.Fprintf(initStderr, "jirafs init: %v\n", err)
		return 1
	}

	if *gitFlag {
		if err := initGitMirror(initStderr, expanded); err != nil {
			fmt.Fprintf(initStderr, "jirafs init: %v\n", err)
			return 1
		}
	}

	if *setCurrentFlag {
		s.State.CurrentProject = name
		if err := s.SaveState(); err != nil {
			fmt.Fprintf(initStderr, "jirafs init: cannot save state: %v\n", err)
			return 1
		}
	}

	fmt.Fprintf(initStdout, "jirafs init: complete — instance %q, project %q (key %q)\n", instance, name, key)
	printInitSummary(initStdout, expanded, created, skipped)
	return 0
}

// initLayout returns the files init writes into the mirror directory. An
// existing mirror.yaml counts as the mirror config, so mirror.yml is omitted.
func initLayout(mirrorDir, key string) []layoutFile {
	var files []layoutFile
	if _, err := os.Stat(filepath.Join(mirrorDir, "mirror.yaml")); err != nil {
		files = append(files, layoutFile{name: "mirror.yml", content: mirrorYAMLContent(key)})
	}
	for _, name := range []string{
		registry.UsersFile, registry.ProjectsFile, registry.StatusesFile,
		registry.SprintsFile, registry.FixVersionsFile, registry.IssueTypesFile,
	} {
		files = append(files, layoutFile{name: name, content: registryEntryContent})
	}
	return files
}

// gitignoreEntries are the derived or transient paths a git-backed mirror
// never tracks. base/ is derived data.
var gitignoreEntries = []string{".jirafs/backup/", ".jirafs/intents/", ".jirafs/tmp/", ".jirafs/base/"}

// initGitMirror makes dir a git-backed mirror: it sets `git: true` in the
// mirror config, runs git init unless dir is already inside a repository
// (its own or a parent's, which is reused), and appends the missing
// gitignore entries to dir/.gitignore without touching existing lines.
func initGitMirror(w io.Writer, dir string) error {
	if err := ensureGitSetting(dir); err != nil {
		return err
	}
	if gitx.IsRepo(dir) {
		top, err := gitx.Toplevel(dir)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "jirafs init: reusing existing git repository at %s\n", top)
	} else {
		if err := gitx.Init(dir); err != nil {
			return err
		}
		fmt.Fprintf(w, "jirafs init: created new git repository in %s\n", dir)
	}
	return ensureGitignore(dir)
}

// ensureGitSetting appends `git: true` to the mirror config unless present.
func ensureGitSetting(dir string) error {
	path := filepath.Join(dir, "mirror.yml")
	if _, err := os.Stat(filepath.Join(dir, "mirror.yaml")); err == nil {
		path = filepath.Join(dir, "mirror.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", path, err)
	}
	m, err := mirror.UnmarshalMirror(data)
	if err != nil {
		return fmt.Errorf("cannot parse %s: %w", path, err)
	}
	if m.Git {
		return nil
	}
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		data = append(data, '\n')
	}
	data = append(data, "git: true\n"...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}

// ensureGitignore appends any missing gitignoreEntries to dir/.gitignore.
func ensureGitignore(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot read %s: %w", path, err)
	}
	have := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		have[strings.TrimSpace(line)] = true
	}
	var add strings.Builder
	for _, e := range gitignoreEntries {
		if !have[e] {
			add.WriteString(e + "\n")
		}
	}
	if add.Len() == 0 {
		return nil
	}
	out := string(data)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if err := os.WriteFile(path, []byte(out+add.String()), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}

// isInteractiveInput reports whether stdin is a terminal. /dev/null is a
// character device but not interactive, so it is excluded.
func isInteractiveInput() bool {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, nerr := os.Stat(os.DevNull); nerr == nil && os.SameFile(info, null) {
		return false
	}
	return true
}

// promptMissingValue asks on stderr for *value when it is empty.
func promptMissingValue(reader *bufio.Reader, value *string, label, description string) {
	if strings.TrimSpace(*value) != "" {
		return
	}
	fmt.Fprintf(initStderr, "jirafs init: enter %s (%s): ", label, description)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	*value = strings.TrimSpace(line)
}

// applyInitDefaults fills the auth type and mirror directory defaults and
// returns a description of each default applied.
func applyInitDefaults(name string, authType, mirrorDir *string) []string {
	var used []string
	if strings.TrimSpace(*authType) == "" {
		*authType = "atlassian_api_token"
		used = append(used, `--auth-type="atlassian_api_token"`)
	}
	if strings.TrimSpace(*mirrorDir) == "" && strings.TrimSpace(name) != "" {
		*mirrorDir = filepath.Join("~", "jira", name)
		used = append(used, fmt.Sprintf(`--mirror-dir=%q`, *mirrorDir))
	}
	return used
}

// missingInitValues names every required value that is empty.
func missingInitValues(name, key, instance, baseURL string) []string {
	var missing []string
	if name == "" {
		missing = append(missing, "<project-name>")
	}
	if key == "" {
		missing = append(missing, "--key")
	}
	if instance == "" {
		missing = append(missing, "--instance")
	}
	if baseURL == "" {
		missing = append(missing, "--base-url")
	}
	return missing
}

// mirrorYAMLContent renders the mirror.yml body for the given Jira project key.
// It matches the shape produced by the setup flow so both paths are compatible.
func mirrorYAMLContent(key string) string {
	return fmt.Sprintf(`project:
  type: project
  value: %s
scopes:
  - name: current-sprint
    type: jql
    target: sprint in openSprints()
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`, key)
}

// ensureLayout writes each file in the mirror directory only when it is
// missing, returning the created and skipped paths.
func ensureLayout(mirrorDir string, files []layoutFile) ([]string, []string, error) {
	var created, skipped []string
	for _, f := range files {
		path := filepath.Join(mirrorDir, f.name)
		if _, err := os.Stat(path); err == nil {
			skipped = append(skipped, path)
			continue
		} else if !os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("cannot check %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(f.content), 0o644); err != nil {
			return nil, nil, fmt.Errorf("cannot write %s: %w", path, err)
		}
		created = append(created, path)
	}
	return created, skipped, nil
}

// printInitSummary writes a concise created/skipped report to w.
func printInitSummary(w io.Writer, mirrorDir string, created, skipped []string) {
	fmt.Fprintf(w, "jirafs init: project layout ready in %s\n", mirrorDir)
	fmt.Fprintf(w, "  created: %d, skipped: %d\n", len(created), len(skipped))
	if len(created) > 0 {
		fmt.Fprintln(w, "\ncreated:")
		for _, p := range created {
			fmt.Fprintf(w, "  %s\n", p)
		}
	}
	if len(skipped) > 0 {
		fmt.Fprintln(w, "\nskipped (already present):")
		for _, p := range skipped {
			fmt.Fprintf(w, "  %s\n", p)
		}
	}
}

// expandTilde expands a leading ~ (and ~/...) to the user's home directory.
func expandTilde(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if p == "~" {
		return home, nil
	}
	if len(p) < 2 || p[1] != '/' {
		return "", fmt.Errorf("unsupported tilde reference %q", p)
	}
	return filepath.Join(home, p[2:]), nil
}

// printInitHelp prints usage information for the init subcommand.
func printInitHelp(fs *flag.FlagSet) {
	PrintUsage(initStdout, Usage{
		Synopsis: []string{"init [flags] <project-name>"},
		Description: []string{
			"Configures one Jira instance and one project: records them in",
			"~/.jirafs/settings.toml, creates the mirror directory and writes",
			"mirror.yml plus the six registry files (users, projects, statuses,",
			"sprints, fix_versions, issue_types). Existing files are left untouched.",
			"Missing values are prompted for in a terminal.",
		},
		Sections: []UsageSection{{Title: "Flags", Rows: append(FlagRows(fs), [2]string{"--help, -h", "show this help message"})}},
	})
}
