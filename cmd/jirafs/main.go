package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jirafs/jirafs/internal/cli"
	"github.com/jirafs/jirafs/internal/color"
	"github.com/jirafs/jirafs/internal/config"
)

// version is set at build time via -ldflags "-X main.version=<value>".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches one jirafs invocation and returns its exit code. It never
// calls os.Exit. Output of cmd/jirafs's own commands goes to stdout/stderr;
// the internal/cli handlers still write to the process-wide os.Stdout and
// os.Stderr, which in-process tests redirect.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printHelp(stdout)
		return 0
	}
	rest := args[1:]
	switch args[0] {
	case "help", "--help", "-h":
		printHelp(stdout)
		return 0
	case "--version", "-v":
		fmt.Fprintln(stdout, "jirafs", version)
		return 0
	case "board":
		return cli.RunBoard(rest)
	case "init":
		return cli.RunInit(rest)
	case "new":
		return cli.RunNew(rest)
	case "push":
		return cli.RunPush(rest)
	case "registry":
		return cli.RunRegistry(rest)
	case "archive":
		return cli.RunArchive(rest)
	case "setup":
		return runSetup(rest, stdout, stderr)
	case "sync":
		return cli.RunSync(rest)
	case "use":
		return runUse(rest, stdout, stderr)
	case "mirror":
		return cli.RunMirror(rest)
	case "export":
		return cli.RunExport(rest)
	case "plan":
		return cli.RunPlan(rest)
	case "status":
		return cli.RunStatus(rest)
	case "doctor":
		return cli.RunDoctor(rest)
	default:
		fmt.Fprintf(stderr, "jirafs: unknown command: %q\nUse --help for usage.\n", args[0])
		return 2
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
func runUse(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("use", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	project := fs.String("project", "", "project key to set as current")
	clear := fs.Bool("clear", false, "clear the remembered current project")
	if hasHelpFlag(args) {
		printUseHelp(stdout, fs)
		return 0
	}
	if err := fs.Parse(args); err != nil {
		return usageErr(stderr, "jirafs use", err.Error())
	}

	// --clear takes priority.
	if *clear {
		if *project != "" {
			return usageErr(stderr, "jirafs use", "--project and --clear are mutually exclusive")
		}
		s, err := config.Load()
		if err != nil {
			fmt.Fprintf(stderr, "jirafs use: cannot load settings: %v\n", err)
			return 1
		}
		s.State.CurrentProject = ""
		if err := s.SaveState(); err != nil {
			fmt.Fprintf(stderr, "jirafs use: cannot save state: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "jirafs: current project cleared")
		return 0
	}

	// Determine the project key from --project flag or positional args.
	var key string
	if *project != "" {
		key = *project
	} else if len(fs.Args()) == 1 {
		key = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		return usageErr(stderr, "jirafs use", "too many positional arguments")
	}

	// No project given → show current state and list other
	// available projects as next-step choices.
	if key == "" {
		s, err := config.Load()
		if err != nil {
			fmt.Fprintf(stderr, "jirafs use: cannot load settings: %v\n", err)
			return 1
		}
		if s.State.CurrentProject == "" {
			fmt.Fprintln(stdout, "jirafs: no current project set")
			if len(s.Projects) > 0 {
				fmt.Fprintln(stderr, "jirafs: available projects:")
				for name := range s.Projects {
					fmt.Fprintf(stderr, "  - %s\n", name)
				}
			}
		} else {
			fmt.Fprintf(stdout, "jirafs: current project is %q\n", s.State.CurrentProject)
			if len(s.Projects) > 1 {
				fmt.Fprintln(stderr, "jirafs: other projects:")
				for name, proj := range s.Projects {
					if name == s.State.CurrentProject {
						continue
					}
					fmt.Fprintf(stderr, "  - %s (key: %s)\n", name, proj.Key)
				}
			}
		}
		return 0
	}

	s, err := config.Load()
	if err != nil {
		fmt.Fprintf(stderr, "jirafs use: cannot load settings: %v\n", err)
		return 1
	}

	if _, ok := s.Projects[key]; !ok {
		fmt.Fprintf(stderr, "jirafs use: project %q not found in settings\n", key)
		return 1
	}

	s.State.CurrentProject = key
	if err := s.SaveState(); err != nil {
		fmt.Fprintf(stderr, "jirafs use: cannot save state: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "jirafs: current project set to %q\n", key)
	return 0
}

// runSetup handles the `jirafs setup` command. It records one named
// instance and one named project into ~/.jirafs/settings.toml, creating
// the file if it does not exist. The caller provides:
//
//   - --project <name>: the settings key for the project entry
//   - --key <KEY>: the Jira project key (e.g. "PLAT")
//   - --instance <name>: the settings key for the instance entry
//   - --base-url <URL>: the Jira base URL
//   - --auth-type <type>: the auth type (basic, atlassian_api_token, bearer_token, oauth1)
//   - --credential-ref <ref>: one credential reference (can be repeated)
//   - --set-current: also set the remembered current project
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
func runSetup(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	projectName := fs.String("project", "", "settings key for the project entry")
	projectKey := fs.String("key", "", "Jira project key (e.g. PLAT)")
	instanceName := fs.String("instance", "", "settings key for the instance entry")
	baseURL := fs.String("base-url", "", "Jira base URL (https://...)")
	authType := fs.String("auth-type", "", "auth type: basic, atlassian_api_token, bearer_token, or oauth1 (default: atlassian_api_token)")
	mirrorDir := fs.String("mirror-dir", "", "local mirror directory path (default: ~/jira/<project>)")
	setCurrent := fs.Bool("set-current", false, "also set the remembered current project")
	var credentialRefs []string
	fs.Func("credential-ref", "credential reference (env://..., file://..., or op://...); may be repeated", func(v string) error {
		credentialRefs = append(credentialRefs, v)
		return nil
	})
	if hasHelpFlag(args) {
		printSetupHelp(stdout, fs)
		return 0
	}
	if err := fs.Parse(args); err != nil {
		return usageErr(stderr, "jirafs setup", err.Error())
	}

	if isInteractiveInput() {
		reader := bufio.NewReader(os.Stdin)
		promptMissingValue(stderr, reader, projectName, "project", "settings key for the project entry")
		promptMissingValue(stderr, reader, projectKey, "key", "Jira project key (for example PLAT)")
		promptMissingValue(stderr, reader, instanceName, "instance", "settings key for the Jira instance")
		promptMissingValue(stderr, reader, baseURL, "base-url", "Jira base URL")
		promptMissingValue(stderr, reader, authType, "auth-type", "auth type (basic, atlassian_api_token, bearer_token, oauth1; default atlassian_api_token)")
		promptMissingValue(stderr, reader, mirrorDir, "mirror-dir", "local mirror directory path (default ~/jira/<project>)")
	}

	defaultsUsed := applySetupDefaults(projectName, authType, mirrorDir)

	if missing := missingSetupFlags(*projectName, *projectKey, *instanceName, *baseURL); len(missing) > 0 {
		return usageErr(stderr, "jirafs setup", "missing required flags: "+strings.Join(missing, ", ")+" (pass them as flags or run setup interactively from a terminal)")
	}
	if *baseURL != "" && (!strings.HasPrefix(*baseURL, "http://") && !strings.HasPrefix(*baseURL, "https://")) {
		return usageErr(stderr, "jirafs setup", "--base-url must start with http:// or https://")
	}

	fmt.Fprintf(
		stderr,
		"jirafs setup: configuring project %q (key %q) on instance %q at %q using auth %q\n",
		*projectName,
		*projectKey,
		*instanceName,
		*baseURL,
		*authType,
	)
	for _, msg := range defaultsUsed {
		fmt.Fprintf(stderr, "jirafs setup: default %s\n", msg)
	}
	fmt.Fprintf(stderr, "jirafs setup: ensuring mirror directory %q exists\n", *mirrorDir)

	// Create or validate the mirror directory before persisting settings.
	if _, err := os.Stat(*mirrorDir); os.IsNotExist(err) || isNotADirectory(err) {
		if err := os.MkdirAll(*mirrorDir, 0o755); err != nil {
			fmt.Fprintf(stderr, "jirafs setup: cannot create mirror directory %q: %v\n", *mirrorDir, err)
			return 1
		}
	}

	// Load existing settings (or create fresh).
	s, err := config.Load()
	if err != nil {
		// If the file doesn't exist, proceed with a fresh settings.
		if !config.IsSettingError(err, config.ErrMissingField) {
			fmt.Fprintf(stderr, "jirafs setup: cannot load settings: %v\n", err)
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
	fmt.Fprintln(stderr, "jirafs setup: writing settings")
	if err := s.SetupProject(*instanceName, *projectName, *projectKey, *baseURL, *authType, *mirrorDir, credentialRefs); err != nil {
		fmt.Fprintf(stderr, "jirafs setup: %v\n", err)
		return 1
	}

	fmt.Fprintln(stderr, "jirafs setup: ensuring default mirror config")
	if err := ensureDefaultMirror(*mirrorDir, *projectKey); err != nil {
		fmt.Fprintf(stderr, "jirafs setup: cannot initialize mirror config: %v\n", err)
		return 1
	}

	// Optionally set the remembered current project.
	if *setCurrent {
		s.State.CurrentProject = *projectName
		if err := s.SaveState(); err != nil {
			fmt.Fprintf(stderr, "jirafs setup: cannot save state: %v\n", err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "jirafs: setup complete — instance %q, project %q (key %q)\n", *instanceName, *projectName, *projectKey)
	return 0
}

func isInteractiveInput() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func promptMissingValue(stderr io.Writer, reader *bufio.Reader, value *string, flagName, description string) {
	if strings.TrimSpace(*value) != "" {
		return
	}

	fmt.Fprintf(stderr, "jirafs setup: enter --%s (%s): ", flagName, description)
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

func printHelp(stderr io.Writer) {
	fmt.Fprintf(stderr, "%s\n", color.BoldBlue(stderr, "Usage:"))
	fmt.Fprintf(stderr, "  %s <command> [arguments]\n\n", color.Bold(stderr, "jirafs"))

	fmt.Fprintf(stderr, "%s:\n", color.BoldGreen(stderr, "Commands"))
	commands := [][2]string{
		{"init", "initialize a new jirafs project in the current directory"},
		{"export", "export Jira issues into local Markdown files"},
		{"plan", "show a sync plan without applying changes"},
		{"setup", "record Jira instance and project settings"},
		{"status", "show workspace status and next-step hints"},
		{"doctor", "run health checks on config, credentials, and live connectivity"},
		{"sync", "apply a sync plan and push changes to Jira (real service path)"},
		{"new", "create a new issue from a template"},
		{"push", "create a local draft issue in Jira (dry run unless --apply)"},
		{"registry", "manage local registry files for typed references"},
		{"board", "show a local kanban-style board view"},
		{"archive", "manage archived issue files"},
		{"mirror", "manage live mirror scopes and archive candidates"},
		{"use", "update remembered project context"},
		{"help", "show this help message"},
	}
	for _, cmd := range commands {
		fmt.Fprintf(stderr, "  %s  %s\n", color.Blue(stderr, cmd[0]), color.Dim(stderr, cmd[1]))
	}

	fmt.Fprintf(stderr, "\n%s\n", color.Cyan(stderr, `Run "jirafs <command> --help" for more information about a command.`))
}

// hasHelpFlag reports whether args ask for help before any "--" terminator.
func hasHelpFlag(args []string) bool {
	for i, a := range args {
		if a == "--" {
			return false
		}
		if a == "--help" || a == "-h" || a == "-help" || (i == 0 && a == "help") {
			return true
		}
	}
	return false
}

// usageErr prints one error line plus the standard hint and returns the
// usage-error exit code 2.
func usageErr(stderr io.Writer, cmd, msg string) int {
	fmt.Fprintf(stderr, "%s: %s\nUse --help for usage.\n", cmd, msg)
	return 2
}

// printUseHelp prints usage information for `jirafs use` to w.
func printUseHelp(w io.Writer, fs *flag.FlagSet) {
	cli.PrintUsage(w, cli.Usage{
		Synopsis: []string{"use [flags] [project]"},
		Description: []string{
			"Shows, sets or clears the remembered current project.",
			"With no arguments it shows the current project.",
		},
		Sections: []cli.UsageSection{{Title: "Flags", Rows: append(cli.FlagRows(fs), [2]string{"--help, -h", "show this help message"})}},
	})
}

// printSetupHelp prints usage information for `jirafs setup` to w.
func printSetupHelp(w io.Writer, fs *flag.FlagSet) {
	cli.PrintUsage(w, cli.Usage{
		Synopsis: []string{"setup [flags]"},
		Description: []string{
			"Records one Jira instance and one project in ~/.jirafs/settings.toml,",
			"creating the file if needed. Missing values are prompted for in a terminal.",
		},
		Sections: []cli.UsageSection{{Title: "Flags", Rows: append(cli.FlagRows(fs), [2]string{"--help, -h", "show this help message"})}},
	})
}
