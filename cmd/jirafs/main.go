package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jirafs/jirafs/internal/cli"
	"github.com/jirafs/jirafs/internal/config"
)

// version is set at build time via -ldflags "-X main.version=<value>".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "help":
		printHelp()
	case "--version", "-v":
		fmt.Println("jirafs", version)
	case "init", "new", "registry", "board", "archive":
		fmt.Fprintf(os.Stderr, "jirafs %s: not yet implemented\n", os.Args[1])
		os.Exit(1)
	case "setup":
		os.Exit(runSetup(os.Args[2:]))
	case "sync":
		os.Exit(cli.RunSync(os.Args[2:]))
	case "use":
		os.Exit(runUse(os.Args[2:]))
	case "mirror":
		os.Exit(cli.RunMirror(os.Args[2:]))
	case "export":
		os.Exit(cli.RunExport(os.Args[2:]))
	case "plan":
		os.Exit(cli.RunPlan(os.Args[2:]))
	case "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "jirafs: unknown command: %q\n\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

// runUse handles the `jirafs use` command. It supports three modes:
//
//  1. `jirafs use` (no args) → show the current project.
//  2. `jirafs use --clear` → clear the remembered current project.
//  3. `jirafs use <project_key>` or `jirafs use --project <KEY>` → set it.
//
// It loads the user's settings, validates the project exists, updates the
// remembered current_project in state, writes it back, and prints a message.
// Returns an exit code (0 on success, 1 on error).
func runUse(args []string) int {
	fs := flag.NewFlagSet("use", flag.ExitOnError)
	project := fs.String("project", "", "project key to set as current")
	clear := fs.Bool("clear", false, "clear the remembered current project")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs use: invalid flags: %v\n", err)
		return 1
	}

	// --clear takes priority.
	if *clear {
		if *project != "" {
			fmt.Fprintf(os.Stderr, "jirafs use: --project and --clear are mutually exclusive\n")
			return 1
		}
		s, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "jirafs use: cannot load settings: %v\n", err)
			return 1
		}
		s.State.CurrentProject = ""
		if err := s.SaveState(); err != nil {
			fmt.Fprintf(os.Stderr, "jirafs use: cannot save state: %v\n", err)
			return 1
		}
		fmt.Println("jirafs: current project cleared")
		return 0
	}

	// Determine the project key from --project flag or positional args.
	var key string
	if *project != "" {
		key = *project
	} else if len(fs.Args()) == 1 {
		key = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintf(os.Stderr, "jirafs use: too many positional arguments\n")
		return 1
	}

	// No project given → show current state.
	if key == "" {
		s, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "jirafs use: cannot load settings: %v\n", err)
			return 1
		}
		if s.State.CurrentProject == "" {
			fmt.Println("jirafs: no current project set")
		} else {
			fmt.Printf("jirafs: current project is %q\n", s.State.CurrentProject)
		}
		return 0
	}

	s, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "jirafs use: cannot load settings: %v\n", err)
		return 1
	}

	if _, ok := s.Projects[key]; !ok {
		fmt.Fprintf(os.Stderr, "jirafs use: project %q not found in settings\n", key)
		return 1
	}

	s.State.CurrentProject = key
	if err := s.SaveState(); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs use: cannot save state: %v\n", err)
		return 1
	}

	fmt.Printf("jirafs: current project set to %q\n", key)
	return 0
}

// runSetup handles the `jirafs setup` command. It records one named
// instance and one named project into ~/.jirafs/settings.toml, creating
// the file if it does not exist. The caller provides:
//
//  - --project <name>: the settings key for the project entry
//  - --key <KEY>: the Jira project key (e.g. "PLAT")
//  - --instance <name>: the settings key for the instance entry
//  - --base-url <URL>: the Jira base URL
//  - --auth-type <type>: the auth type (basic, atlassian_api_token, oauth1)
//  - --credential-ref <ref>: one credential reference (can be repeated)
//  - --set-current: also set the remembered current project (B018d)
//
// Example:
//
//	jirafs setup --project platform --key PLAT --instance work \\
//	  --base-url https://jira.example.com --auth-type atlassian_api_token \\
//	  --credential-ref env://JIRAFS_API_TOKEN
//
// With --set-current, the setup command also records the project as the
// remembered current project so the first export/refresh flow works
// without extra flags.
func runSetup(args []string) int {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	projectName := fs.String("project", "", "settings key for the project entry")
	projectKey := fs.String("key", "", "Jira project key (e.g. PLAT)")
	instanceName := fs.String("instance", "", "settings key for the instance entry")
	baseURL := fs.String("base-url", "", "Jira base URL (https://...)")
	authType := fs.String("auth-type", "", "auth type: basic, atlassian_api_token, or oauth1 (default: atlassian_api_token)")
	mirrorDir := fs.String("mirror-dir", "", "local mirror directory path (default: ~/jira/<project>)")
	setCurrent := fs.Bool("set-current", false, "also set the remembered current project (B018d)")
	var credentialRefs []string
	fs.Func("credential-ref", "credential reference (env://... or file://...); may be repeated", func(v string) error {
		credentialRefs = append(credentialRefs, v)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs setup: invalid flags: %v\n", err)
		return 1
	}

	if isInteractiveInput() {
		reader := bufio.NewReader(os.Stdin)
		promptMissingValue(reader, projectName, "project", "settings key for the project entry")
		promptMissingValue(reader, projectKey, "key", "Jira project key (for example PLAT)")
		promptMissingValue(reader, instanceName, "instance", "settings key for the Jira instance")
		promptMissingValue(reader, baseURL, "base-url", "Jira base URL")
		promptMissingValue(reader, authType, "auth-type", "auth type (basic, atlassian_api_token, oauth1; default atlassian_api_token)")
		promptMissingValue(reader, mirrorDir, "mirror-dir", "local mirror directory path (default ~/jira/<project>)")
	}

	defaultsUsed := applySetupDefaults(projectName, authType, mirrorDir)

	if missing := missingSetupFlags(*projectName, *projectKey, *instanceName, *baseURL); len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "jirafs setup: missing required flags: %s\n", strings.Join(missing, ", "))
		fmt.Fprintln(os.Stderr, "jirafs setup: pass them as flags or run setup interactively from a terminal")
		return 1
	}
	if *baseURL != "" && (!strings.HasPrefix(*baseURL, "http://") && !strings.HasPrefix(*baseURL, "https://")) {
		fmt.Fprintf(os.Stderr, "jirafs setup: --base-url must start with http:// or https://\n")
		return 1
	}

	fmt.Fprintf(
		os.Stderr,
		"jirafs setup: configuring project %q (key %q) on instance %q at %q using auth %q\n",
		*projectName,
		*projectKey,
		*instanceName,
		*baseURL,
		*authType,
	)
	for _, msg := range defaultsUsed {
		fmt.Fprintf(os.Stderr, "jirafs setup: default %s\n", msg)
	}
	fmt.Fprintf(os.Stderr, "jirafs setup: ensuring mirror directory %q exists\n", *mirrorDir)

	// Create or validate the mirror directory before persisting settings.
	if _, err := os.Stat(*mirrorDir); os.IsNotExist(err) || isNotADirectory(err) {
		if err := os.MkdirAll(*mirrorDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "jirafs setup: cannot create mirror directory %q: %v\n", *mirrorDir, err)
			return 1
		}
	}

	// Load existing settings (or create fresh).
	s, err := config.Load()
	if err != nil {
		// If the file doesn't exist, proceed with a fresh settings.
		if !config.IsSettingError(err, config.ErrMissingField) {
			fmt.Fprintf(os.Stderr, "jirafs setup: cannot load settings: %v\n", err)
			return 1
		}
		// Missing file is OK — we'll create it.
		s = &config.Settings{
			Version:   1,
			Instances: make(map[string]config.Instance),
			Projects:  make(map[string]config.Project),
		}
	}

	// Call SetupProject to record the instance and project.
	fmt.Fprintln(os.Stderr, "jirafs setup: writing settings")
	if err := s.SetupProject(*instanceName, *projectName, *projectKey, *baseURL, *authType, *mirrorDir, credentialRefs); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs setup: %v\n", err)
		return 1
	}

	fmt.Fprintln(os.Stderr, "jirafs setup: ensuring default mirror config")
	if err := ensureDefaultMirror(*mirrorDir, *projectKey); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs setup: cannot initialize mirror config: %v\n", err)
		return 1
	}

	// Optionally set the remembered current project (B018d).
	if *setCurrent {
		s.State.CurrentProject = *projectName
		if err := s.SaveState(); err != nil {
			fmt.Fprintf(os.Stderr, "jirafs setup: cannot save state: %v\n", err)
			return 1
		}
	}

	fmt.Printf("jirafs: setup complete — instance %q, project %q (key %q)\n", *instanceName, *projectName, *projectKey)
	return 0
}

func isInteractiveInput() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func promptMissingValue(reader *bufio.Reader, value *string, flagName, description string) {
	if strings.TrimSpace(*value) != "" {
		return
	}

	fmt.Fprintf(os.Stderr, "jirafs setup: enter --%s (%s): ", flagName, description)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	*value = strings.TrimSpace(line)
}

func applySetupDefaults(projectName, authType, mirrorDir *string) []string {
	var defaultsUsed []string

	if strings.TrimSpace(*authType) == "" {
		*authType = "atlassian_api_token"
		defaultsUsed = append(defaultsUsed, `--auth-type="atlassian_api_token"`)
	}

	if strings.TrimSpace(*mirrorDir) == "" && strings.TrimSpace(*projectName) != "" {
		*mirrorDir = filepath.Join("~", "jira", *projectName)
		defaultsUsed = append(defaultsUsed, fmt.Sprintf(`--mirror-dir=%q`, *mirrorDir))
	}

	return defaultsUsed
}

func missingSetupFlags(projectName, projectKey, instanceName, baseURL string) []string {
	var missing []string
	if projectName == "" {
		missing = append(missing, "--project")
	}
	if projectKey == "" {
		missing = append(missing, "--key")
	}
	if instanceName == "" {
		missing = append(missing, "--instance")
	}
	if baseURL == "" {
		missing = append(missing, "--base-url")
	}
	return missing
}

func ensureDefaultMirror(mirrorDir, projectKey string) error {
	for _, name := range []string{"mirror.yml", "mirror.yaml"} {
		path := filepath.Join(mirrorDir, name)
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}

	content := fmt.Sprintf(`project:
  type: project
  value: %s
scopes:
  - name: current-sprint
    type: jql
    target: sprint in openSprints()
  - name: my-issues
    type: jql
    target: assignee = currentUser()
`, projectKey)
	path := filepath.Join(mirrorDir, "mirror.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	return nil
}

// isNotADirectory reports whether err is a "not a directory" error.
// This happens when os.Stat encounters a path where a parent component
// is a file (e.g. "file/child" where "file" is a regular file).
func isNotADirectory(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not a directory")
}

func printHelp() {
	fmt.Fprintln(os.Stderr, `Usage:
  jirafs <command> [arguments]

Commands:
  init       initialize a new jirafs project in the current directory
  export     export Jira issues into local Markdown files
  plan       show a sync plan without applying changes
  setup      record Jira instance and project settings
  sync       apply a sync plan and push changes to Jira (real service path)
  new        create a new issue from a template
  registry   manage local registry files for typed references
  board      show a local kanban-style board view
  archive    manage archived issue files
  mirror     manage live mirror scopes and archive candidates
  use        update remembered project context
  help       show this help message

Run "jirafs <command> --help" for more information about a command.`)
}
