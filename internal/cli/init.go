// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
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

// RunInit handles the `jirafs init` command. It creates the local project
// layout in a mirror directory: mirror.yml plus the six registry files. The
// command is idempotent — existing files are never overwritten, and each is
// reported as created or skipped. With --set-current it also records the
// project as the remembered current project in ~/.jirafs/settings.toml.
func RunInit(args []string) int {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "help" {
			printInitHelp()
			return 0
		}
	}

	// init can create a project layout even when settings do not exist yet;
	// settings are only needed to auto-resolve --project from the cwd or to
	// honor --set-current. A missing or unreadable settings file must not block
	// initializing a brand-new project, so only keep settings on success.
	var settings *config.Settings
	if s, err := config.Load(); err == nil {
		settings = s
	}

	return runInit(args, settings)
}

func runInit(args []string, settings *config.Settings) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(initStderr)
	projectFlag := fs.String("project", "", "settings key for the project (required unless run inside a mapped dir)")
	keyFlag := fs.String("key", "", "Jira project key (e.g. PLAT)")
	instanceFlag := fs.String("instance", "", "Jira instance name")
	jiraURLFlag := fs.String("jira-url", "", "Jira base URL (http:// or https://)")
	mirrorDirFlag := fs.String("mirror-dir", "", "mirror directory (default ~/jira/<project>)")
	setCurrentFlag := fs.Bool("set-current", false, "also set the remembered current project")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(initStderr, "jirafs init: invalid flags: %v\n", err)
		return 1
	}

	// Validate the Jira URL when provided.
	if *jiraURLFlag != "" && !strings.HasPrefix(*jiraURLFlag, "http://") && !strings.HasPrefix(*jiraURLFlag, "https://") {
		fmt.Fprintf(initStderr, "jirafs init: --jira-url must start with http:// or https://\n")
		fmt.Fprintln(initStderr, "jirafs init: pass a valid --jira-url such as https://jira.example.com")
		return 1
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(initStderr, "jirafs init: cannot determine working directory: %v\n", err)
		return 1
	}

	// Best-effort resolution of --project from the cwd mapping.
	project := *projectFlag
	if project == "" && settings != nil {
		if ctx, rerr := context.NewResolver(settings, "").Resolve(cwd); rerr == nil && ctx != nil {
			project = ctx.Name
		}
	}
	if project == "" {
		fmt.Fprintf(initStderr, "jirafs init: no project specified and none resolved for cwd %q\n", cwd)
		fmt.Fprintln(initStderr, "jirafs init: run jirafs init --project NAME to initialize a new project")
		return 1
	}

	// Determine the mirror directory: explicit flag, else ~/jira/<project>.
	mirrorDir := *mirrorDirFlag
	if mirrorDir == "" {
		mirrorDir = filepath.Join("~", "jira", project)
	}
	expanded, err := expandTilde(mirrorDir)
	if err != nil {
		fmt.Fprintf(initStderr, "jirafs init: cannot expand mirror directory %q: %v\n", mirrorDir, err)
		return 1
	}
	mirrorDir = expanded

	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		fmt.Fprintf(initStderr, "jirafs init: cannot create mirror directory %q: %v\n", mirrorDir, err)
		return 1
	}

	files := []layoutFile{
		{name: "mirror.yml", content: mirrorYAMLContent(*keyFlag)},
		{name: registry.UsersFile, content: registryEntryContent},
		{name: registry.ProjectsFile, content: registryEntryContent},
		{name: registry.StatusesFile, content: registryEntryContent},
		{name: registry.SprintsFile, content: registryEntryContent},
		{name: registry.FixVersionsFile, content: registryEntryContent},
		{name: registry.IssueTypesFile, content: registryEntryContent},
	}

	created, skipped, err := ensureLayout(mirrorDir, files)
	if err != nil {
		fmt.Fprintf(initStderr, "jirafs init: %v\n", err)
		return 1
	}

	// --set-current is best effort: report the layout regardless, then surface
	// any failure so the caller still learns the command did not fully succeed.
	var setCurrentErr error
	if *setCurrentFlag {
		setCurrentErr = setCurrentProject(settings, project, mirrorDir, *keyFlag, *instanceFlag, *jiraURLFlag)
	}

	printInitSummary(initStdout, mirrorDir, created, skipped)

	if setCurrentErr != nil {
		fmt.Fprintf(initStderr, "jirafs init: cannot set current project: %v\n", setCurrentErr)
		return 1
	}

	return 0
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

// setCurrentProject records the project as the remembered current project.
// When settings do not exist yet it mirrors the setup flow: it records one
// instance and one project before setting current.
func setCurrentProject(settings *config.Settings, project, mirrorDir, key, instance, jiraURL string) error {
	if settings != nil {
		settings.State.CurrentProject = project
		return settings.SaveState()
	}

	s := &config.Settings{
		Version:   1,
		Instances: make(map[string]config.Instance),
		Projects:  make(map[string]config.Project),
	}
	if err := s.SetupProject(instance, project, key, jiraURL, "atlassian_api_token", mirrorDir, nil); err != nil {
		return err
	}
	s.State.CurrentProject = project
	return s.SaveState()
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
func printInitHelp() {
	fmt.Fprintf(initStderr, "Usage: jirafs init [flags]\n\n")
	fmt.Fprintf(initStderr, "Initialize a new jirafs project layout in a mirror directory. Creates\n")
	fmt.Fprintf(initStderr, "mirror.yml and the six registry files (users, projects, statuses,\n")
	fmt.Fprintf(initStderr, "sprints, fix_versions, issue_types). Existing files are left untouched.\n\n")
	fmt.Fprintf(initStderr, "Flags:\n")
	fmt.Fprintf(initStderr, "  --project NAME     settings key for the project (required unless run inside a mapped dir)\n")
	fmt.Fprintf(initStderr, "  --key KEY          Jira project key (e.g. PLAT)\n")
	fmt.Fprintf(initStderr, "  --instance NAME    Jira instance name\n")
	fmt.Fprintf(initStderr, "  --jira-url URL     Jira base URL (http:// or https://)\n")
	fmt.Fprintf(initStderr, "  --mirror-dir PATH  mirror directory (default ~/jira/<project>)\n")
	fmt.Fprintf(initStderr, "  --set-current      also set the remembered current project\n")
}
