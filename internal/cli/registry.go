// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/jirafs/jirafs/internal/color"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/registry"
)

var (
	registryStdout io.Writer = os.Stdout
	registryStderr io.Writer = os.Stderr
)

// RunRegistry dispatches the `jirafs registry` subcommand. It manages local
// typed-reference registry files (users, projects, statuses, sprints, and
// fix_versions) that live in the active project's mirror directory. All
// operations are offline except refresh, which requires a live Jira
// connection and is therefore always an error in this offline build.
//
// Supported subcommands:
//
//	"jirafs registry show <type> [--json]"  → display a registry
//	"jirafs registry init <type>"           → create an empty registry file
//	"jirafs registry refresh <type>"        → populate from Jira (offline: error)
//	"jirafs registry help"                  → show this help
func RunRegistry(args []string) int {
	if len(args) == 0 {
		return usageError(registryStderr, "jirafs registry", "missing subcommand")
	}
	if wantsHelpOrWord(args[:1]) {
		printRegistryHelp()
		return 0
	}

	if wantsHelp(args[1:]) {
		printRegistryHelp()
		return 0
	}

	switch args[0] {
	case "show":
		return runRegistryShow(args[1:])
	case "init":
		return runRegistryInit(args[1:])
	case "refresh":
		return runRegistryRefresh(args[1:])
	default:
		return usageError(registryStderr, "jirafs registry", "unknown subcommand %q", args[0])
	}
}

// runRegistryShow handles `jirafs registry show <type> [--json]`.
func runRegistryShow(args []string) int {
	project, cwd, jsonOut, positionals, err := parseRegistryFlags(args, true)
	if err != nil {
		return flagError(registryStderr, "jirafs registry show", err)
	}
	if len(positionals) == 0 {
		return usageError(registryStderr, "jirafs registry show", "missing <type>")
	}
	typ := positionals[0]

	file, ok := registryFileFor(typ)
	if !ok {
		fmt.Fprintf(registryStderr, "jirafs registry show: unknown type %q; use users, projects, statuses, sprints, or fix_versions\n", typ)
		return 1
	}

	settings, ok := loadSettingsOrHint(registryStderr, "registry show")
	if !ok {
		return 1
	}

	ctx, ok := resolveRegistryContext(settings, project, cwd)
	if !ok {
		return 1
	}

	path := filepath.Join(ctx.MirrorDir, file)
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			fmt.Fprintf(registryStderr, "registry %s not found; run 'jirafs registry init %s'\n", typ, typ)
			return 1
		}
		fmt.Fprintf(registryStderr, "jirafs registry show %s: %v\n", typ, statErr)
		return 1
	}

	switch typ {
	case "users":
		entries, loadErr := registry.LoadUsers(ctx.MirrorDir)
		if loadErr != nil {
			fmt.Fprintf(registryStderr, "jirafs registry show users: %v\n", loadErr)
			return 1
		}
		return renderRegistry[string, registry.User](entries, func(u registry.User) string { return u.DisplayName }, jsonOut)
	case "projects":
		entries, loadErr := registry.LoadProjects(ctx.MirrorDir)
		if loadErr != nil {
			fmt.Fprintf(registryStderr, "jirafs registry show projects: %v\n", loadErr)
			return 1
		}
		return renderRegistry[string, registry.Project](entries, func(p registry.Project) string { return p.Key }, jsonOut)
	case "statuses":
		entries, loadErr := registry.LoadStatuses(ctx.MirrorDir)
		if loadErr != nil {
			fmt.Fprintf(registryStderr, "jirafs registry show statuses: %v\n", loadErr)
			return 1
		}
		return renderRegistry[string, registry.Status](entries, func(s registry.Status) string { return s.Name }, jsonOut)
	case "sprints":
		entries, loadErr := registry.LoadSprints(ctx.MirrorDir)
		if loadErr != nil {
			fmt.Fprintf(registryStderr, "jirafs registry show sprints: %v\n", loadErr)
			return 1
		}
		return renderRegistry[string, registry.Sprint](entries, func(s registry.Sprint) string { return s.Name }, jsonOut)
	case "fix_versions":
		entries, loadErr := registry.LoadFixVersions(ctx.MirrorDir)
		if loadErr != nil {
			fmt.Fprintf(registryStderr, "jirafs registry show fix_versions: %v\n", loadErr)
			return 1
		}
		return renderRegistry[string, registry.FixVersion](entries, func(f registry.FixVersion) string { return f.Name }, jsonOut)
	}
	return 1
}

// runRegistryInit handles `jirafs registry init <type>`. It creates the
// registry file if missing with loadable `entries: {}` content; if the file
// already exists it reports a skip and returns success.
func runRegistryInit(args []string) int {
	project, cwd, _, positionals, err := parseRegistryFlags(args, false)
	if err != nil {
		return flagError(registryStderr, "jirafs registry init", err)
	}
	if len(positionals) == 0 {
		return usageError(registryStderr, "jirafs registry init", "missing <type>")
	}
	typ := positionals[0]

	file, ok := registryFileFor(typ)
	if !ok {
		fmt.Fprintf(registryStderr, "jirafs registry init: unknown type %q; use users, projects, statuses, sprints, or fix_versions\n", typ)
		return 1
	}

	settings, ok := loadSettingsOrHint(registryStderr, "registry init")
	if !ok {
		return 1
	}

	ctx, ok := resolveRegistryContext(settings, project, cwd)
	if !ok {
		return 1
	}

	path := filepath.Join(ctx.MirrorDir, file)
	if _, statErr := os.Stat(path); statErr == nil {
		fmt.Fprintf(registryStdout, "already exists, skipping\n")
		return 0
	} else if !os.IsNotExist(statErr) {
		fmt.Fprintf(registryStderr, "jirafs registry init %s: %v\n", typ, statErr)
		return 1
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(registryStderr, "jirafs registry init %s: cannot create directory: %v\n", typ, err)
		return 1
	}
	if err := os.WriteFile(path, []byte("entries: {}\n"), 0o644); err != nil {
		fmt.Fprintf(registryStderr, "jirafs registry init %s: cannot create %s: %v\n", typ, file, err)
		return 1
	}
	fmt.Fprintf(registryStdout, "created %s\n", path)
	return 0
}

// runRegistryRefresh handles `jirafs registry refresh <type>`. Refreshing a
// registry requires reading live data from Jira, which is unavailable in an
// offline build, so this always reports the situation and exits non-zero
// without performing any network operation.
func runRegistryRefresh(args []string) int {
	_, _, _, positionals, err := parseRegistryFlags(args, false)
	if err != nil {
		return flagError(registryStderr, "jirafs registry refresh", err)
	}
	if len(positionals) == 0 {
		return usageError(registryStderr, "jirafs registry refresh", "missing <type>")
	}
	typ := positionals[0]

	fmt.Fprintf(registryStderr, "jirafs registry refresh %s: requires a live Jira connection; not available offline. Run 'jirafs mirror refresh <scope>' to populate registries from Jira.\n", typ)
	return 1
}

// parseRegistryFlags parses the shared registry flags (--project, --cwd, and
// --json when withJSON is true). Flags may appear before or after the
// positional <type> argument, so flags are extracted in a loop that stops at
// each positional and resumes parsing the remainder.
func parseRegistryFlags(args []string, withJSON bool) (project, cwd string, jsonOut bool, positionals []string, callErr error) {
	fs := quietFlagSet("registry")

	jsonFlag := fs.Bool("json", false, "output registries as JSON (show)")
	projectFlag := fs.String("project", "", "name of a configured project")
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")

	positionals, callErr = parseInterspersed(fs, args)
	if callErr != nil {
		return "", "", false, nil, callErr
	}
	return *projectFlag, *cwdFlag, *jsonFlag, positionals, nil
}

// registryFileFor maps a registry type name to its file constant. The second
// return value reports whether typ is a known registry type.
func registryFileFor(typ string) (string, bool) {
	switch typ {
	case "users":
		return registry.UsersFile, true
	case "projects":
		return registry.ProjectsFile, true
	case "statuses":
		return registry.StatusesFile, true
	case "sprints":
		return registry.SprintsFile, true
	case "fix_versions":
		return registry.FixVersionsFile, true
	default:
		return "", false
	}
}

// renderRegistry renders a typed registry map to stdout, either as indented
// JSON (--json) or as one "  <ref>  <summary>" line per entry, sorted by ref.
// summary extracts the human-readable summary from a value; the caller
// supplies it because each registry value type has a different summary field.
func renderRegistry[K ~string, V any](entries map[K]V, summary func(V) string, jsonOut bool) int {
	if jsonOut {
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			fmt.Fprintf(registryStderr, "jirafs registry show: cannot encode JSON: %v\n", err)
			return 1
		}
		fmt.Fprint(registryStdout, string(data), "\n")
		return 0
	}

	if len(entries) == 0 {
		fmt.Fprintln(registryStdout, "  (empty)")
		return 0
	}

	keys := make([]K, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		fmt.Fprintf(registryStdout, "  %v  %s\n", k, summary(entries[k]))
	}
	return 0
}

// resolveRegistryContext resolves the active project context for registry
// operations. It mirrors resolveMirrorContext but emits registry-named
// messages: resolveMirrorContext is hardcoded to "jirafs mirror …" output and
// lives in a file this command must not edit. On failure it prints an error
// plus a next-step hint and returns false.
func resolveRegistryContext(settings *config.Settings, project, cwd string) (*context.Context, bool) {
	res := context.NewResolver(settings, project)
	ctx, err := res.Resolve(cwd)
	if err != nil {
		var ce *context.Error
		if context.IsContextError(err, &ce) {
			if ce.Code == config.ErrNoProjectResolved {
				fmt.Fprintf(registryStderr, "jirafs registry: no project resolved for cwd %q\n", cwd)
				fmt.Fprintln(registryStderr, `run "jirafs use --project NAME" or "jirafs init <name> ..."`)
				if len(ce.Candidates) > 0 {
					fmt.Fprintln(registryStderr, "Available projects:")
					for _, name := range ce.Candidates {
						fmt.Fprintf(registryStderr, "  - %s\n", name)
					}
				}
				return nil, false
			}
			fmt.Fprintf(registryStderr, "jirafs registry: %v\n", err)
			return nil, false
		}
		fmt.Fprintf(registryStderr, "jirafs registry: %v\n", err)
		return nil, false
	}
	return ctx, true
}

// printRegistryHelp prints usage information for the registry subcommand.
func printRegistryHelp() {
	fmt.Fprintf(registryStdout, "%s\n", color.BoldBlue(registryStdout, "Usage:"))
	fmt.Fprintf(registryStdout, "  jirafs %s [flags] <subcommand> [args]\n\n", color.Blue(registryStdout, "registry"))

	fmt.Fprintf(registryStdout, "%s\n", color.Dim(registryStdout, "Manage local typed-reference registry files (users, projects, statuses,"))
	fmt.Fprintf(registryStdout, "%s\n\n", color.Dim(registryStdout, "sprints, fix_versions). These live in the active project's mirror directory."))

	fmt.Fprintf(registryStdout, "%s:\n", color.BoldGreen(registryStdout, "Subcommands"))
	fmt.Fprintf(registryStdout, "  %s   %s\n", color.Yellow(registryStdout, "show <type>"), color.Dim(registryStdout, "display a registry; add --json for JSON output"))
	fmt.Fprintf(registryStdout, "  %s   %s\n", color.Yellow(registryStdout, "init <type>"), color.Dim(registryStdout, "create an empty registry file"))
	fmt.Fprintf(registryStdout, "  %s   %s\n\n", color.Yellow(registryStdout, "refresh <type>"), color.Dim(registryStdout, "populate a registry from Jira (requires network access)"))

	fmt.Fprintf(registryStdout, "%s:\n", color.BoldGreen(registryStdout, "Types"))
	fmt.Fprintf(registryStdout, "  %s\n\n", color.Dim(registryStdout, "users, projects, statuses, sprints, fix_versions"))

	fmt.Fprintf(registryStdout, "%s:\n", color.BoldGreen(registryStdout, "Flags"))
	fmt.Fprintf(registryStdout, "  %s   %s\n", color.Yellow(registryStdout, "--project"), color.Dim(registryStdout, "name of a configured project"))
	fmt.Fprintf(registryStdout, "  %s   %s\n", color.Yellow(registryStdout, "--cwd"), color.Dim(registryStdout, "working directory for project resolution"))
	fmt.Fprintf(registryStdout, "  %s   %s\n", color.Yellow(registryStdout, "--json"), color.Dim(registryStdout, "output registries as JSON (show)"))
}
