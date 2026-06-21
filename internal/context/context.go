package context

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jirafs/jirafs/internal/config"
)

// PromptReader abstracts interactive prompting for project selection.
type PromptReader interface {
	// PromptSelect displays a prompt and returns the user's selection index
	// (0-based) from the candidates list. It returns -1 if the user
	// cancels (e.g. Ctrl-C or entering "q").
	PromptSelect(prompt string, candidates []string) (int, error)
}

// StdinPromptReader implements PromptReader using os.Stdin and os.Stdout.
type StdinPromptReader struct{}

// PromptSelect implements PromptReader.
func (r *StdinPromptReader) PromptSelect(prompt string, candidates []string) (int, error) {
	fmt.Fprintln(os.Stdout, prompt)
	for i, c := range candidates {
		fmt.Fprintf(os.Stdout, "  %d) %s\n", i+1, c)
	}
	fmt.Fprint(os.Stdout, "Select project [1-"+fmt.Sprint(len(candidates))+"] (q to quit): ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return -1, fmt.Errorf("context: no input from stdin")
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "q" || input == "Q" {
		return -1, nil
	}

	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil {
		return -1, fmt.Errorf("context: invalid input %q", input)
	}
	if idx < 1 || idx > len(candidates) {
		return -1, fmt.Errorf("context: selection out of range [1-%d]", len(candidates))
	}
	return idx - 1, nil
}

// Context holds the resolved project information.
type Context struct {
	// Name is the config name of the resolved project.
	Name string

	// Key is the Jira project key.
	Key string

	// MirrorDir is the expanded mirror directory path.
	MirrorDir string

	// Instance is the Jira instance name.
	Instance string
}

// Error is returned when project resolution fails.
type Error struct {
	Code       string
	Message    string
	Candidates []string // optional: project names that were considered
}

func (e *Error) Error() string {
	return "jirafs/context: " + e.Code + ": " + e.Message
}

// NewError creates a new Error with the given code, message, and optional
// candidates. Candidates are known project names that help the user
// understand what is available when resolution fails.
func NewError(code, message string, candidates ...string) *Error {
	e := &Error{Code: code, Message: message}
	if len(candidates) > 0 {
		e.Candidates = candidates
	}
	return e
}

// Resolver resolves the active project from multiple sources.
type Resolver struct {
	settings *config.Settings
	explicit string // explicit --project flag value
}

// NewResolver creates a new Resolver.
func NewResolver(settings *config.Settings, explicit string) *Resolver {
	return &Resolver{
		settings: settings,
		explicit: explicit,
	}
}

// Resolve returns the active project context. It walks the sources in
// precedence order: explicit flag, cwd mapping, remembered state.
func (r *Resolver) Resolve(cwd string) (*Context, error) {
	// 1. Explicit --project flag (highest precedence).
	if r.explicit != "" {
		return r.resolveExplicit()
	}

	// 2. Cwd mapping (most-specific match).
	if ctx, err := r.resolveCwd(cwd); err == nil {
		return ctx, nil
	} else if isAmbiguous(err) {
		// Preserve ambiguity errors; do not fall through to state.
		return nil, err
	}

	// 3. Remembered state (lowest precedence).
	return r.resolveState()
}

// resolveExplicit looks up the project by explicit flag value.
// The value can be either the config name or the Jira project key.
func (r *Resolver) resolveExplicit() (*Context, error) {
	// Try config name first.
	if proj, ok := r.settings.Projects[r.explicit]; ok {
		return &Context{
			Name:      r.explicit,
			Key:       proj.Key,
			MirrorDir: proj.MirrorDir,
			Instance:  proj.Instance,
		}, nil
	}

	// Try Jira project key.
	for name, proj := range r.settings.Projects {
		if proj.Key == r.explicit {
			return &Context{
				Name:      name,
				Key:       proj.Key,
				MirrorDir: proj.MirrorDir,
				Instance:  proj.Instance,
			}, nil
		}
	}

	return nil, NewError(config.ErrUnknownProject,
		fmt.Sprintf("project %q not found in settings", r.explicit))
}

// resolveCwd maps the current working directory to a project by matching
// against mirror_dir and local_dirs prefixes. Returns the most-specific
// (longest prefix) match. If two different projects match at the same
// depth, returns an ErrAmbiguousMatch error with candidates.
func (r *Resolver) resolveCwd(cwd string) (*Context, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, NewError(config.ErrMissingField,
			fmt.Sprintf("cannot resolve current directory: %s", cwd))
	}

	// Normalize for prefix matching.
	abs = filepath.Clean(abs)

	var best *Context
	var bestLen int

	for name, proj := range r.settings.Projects {
		// Check mirror_dir.
		mirror := filepath.Clean(proj.MirrorDir)
		if isPrefixOf(mirror, abs) {
			d := depth(mirror)
			if d > bestLen {
				bestLen = d
				best = &Context{
					Name:      name,
					Key:       proj.Key,
					MirrorDir: proj.MirrorDir,
					Instance:  proj.Instance,
				}
			} else if d == bestLen && best != nil && best.Name != name {
				// Ambiguity: two different projects at the same depth.
				return nil, NewError(config.ErrAmbiguousMatch,
					fmt.Sprintf("multiple projects match cwd %q at depth %d: %q and %q",
						cwd, d, best.Name, name))
			}
		}

		// Check local_dirs.
		for _, ld := range proj.LocalDirs {
			local := filepath.Clean(ld)
			if isPrefixOf(local, abs) {
				d := depth(local)
				if d > bestLen {
					bestLen = d
					best = &Context{
						Name:      name,
						Key:       proj.Key,
						MirrorDir: proj.MirrorDir,
						Instance:  proj.Instance,
					}
				} else if d == bestLen && best != nil && best.Name != name {
					// Ambiguity: two different projects at the same depth.
					return nil, NewError(config.ErrAmbiguousMatch,
						fmt.Sprintf("multiple projects match cwd %q at depth %d: %q and %q",
							cwd, d, best.Name, name))
				}
			}
		}
	}

	if best == nil {
		candidates := make([]string, 0, len(r.settings.Projects))
		for name := range r.settings.Projects {
			candidates = append(candidates, name)
		}
		return nil, NewError(config.ErrNoProjectResolved,
			fmt.Sprintf("no project matches cwd %q", cwd), candidates...)
	}

	return best, nil
}

// resolveState returns the remembered project from settings state.
func (r *Resolver) resolveState() (*Context, error) {
	stateName := r.settings.State.CurrentProject
	if stateName == "" {
		candidates := make([]string, 0, len(r.settings.Projects))
		for name := range r.settings.Projects {
			candidates = append(candidates, name)
		}
		return nil, NewError(config.ErrNoProjectResolved,
			"no project configured and no remembered project", candidates...)
	}

	proj, ok := r.settings.Projects[stateName]
	if !ok {
		return nil, NewError(config.ErrUnknownProject,
			fmt.Sprintf("remembered project %q not found in settings", stateName))
	}

	return &Context{
		Name:      stateName,
		Key:       proj.Key,
		MirrorDir: proj.MirrorDir,
		Instance:  proj.Instance,
	}, nil
}

// SaveCurrentProject persists the given project name as the remembered
// current project in the settings state.
func (r *Resolver) SaveCurrentProject(name string) error {
	r.settings.State.CurrentProject = name
	return r.settings.SaveState()
}

// InteractiveResolve resolves the active project context. If normal
// resolution fails with ErrNoProjectResolved, it interactively prompts
// the user to select from the known project candidates.
func (r *Resolver) InteractiveResolve(cwd string, prompter PromptReader) (*Context, error) {
	ctx, err := r.Resolve(cwd)
	if err == nil {
		return ctx, nil
	}

	var ce *Error
	if !isContextError(err, &ce) {
		return nil, err
	}
	if ce.Code != config.ErrNoProjectResolved {
		return nil, err
	}

	if prompter == nil {
		prompter = &StdinPromptReader{}
	}

	// Build a stable candidate list from the error.
	candidates := ce.Candidates
	if len(candidates) == 0 {
		// Fallback: collect from settings.
		candidates = make([]string, 0, len(r.settings.Projects))
		for name := range r.settings.Projects {
			candidates = append(candidates, name)
		}
		sort.Strings(candidates)
	}

	idx, err := prompter.PromptSelect("No project resolved for the current context.\nSelect a project:", candidates)
	if err != nil {
		return nil, fmt.Errorf("context: interactive prompt failed: %w", err)
	}
	if idx == -1 {
		return nil, fmt.Errorf("context: no project selected")
	}

	selected := candidates[idx]

	// Try to resolve the selected project.
	if proj, ok := r.settings.Projects[selected]; ok {
		if err := r.SaveCurrentProject(selected); err != nil {
			return nil, fmt.Errorf("context: failed to save selected project %q: %w", selected, err)
		}
		return &Context{
			Name:      selected,
			Key:       proj.Key,
			MirrorDir: proj.MirrorDir,
			Instance:  proj.Instance,
		}, nil
	}

	return nil, NewError(config.ErrUnknownProject,
		fmt.Sprintf("selected project %q not found in settings", selected))
}

// isContextError unwraps err looking for a *Error and assigns it to target.
func isContextError(err error, target **Error) bool {
	if err == nil {
		return false
	}
	for err != nil {
		if t, ok := err.(*Error); ok {
			*target = t
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
			continue
		}
		break
	}
	return false
}

// isAmbiguous reports whether err is an ErrAmbiguousMatch error.
func isAmbiguous(err error) bool {
	if err == nil {
		return false
	}
	var ce *Error
	for err != nil {
		if target, ok := err.(*Error); ok {
			ce = target
			break
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
			continue
		}
		break
	}
	return ce != nil && ce.Code == config.ErrAmbiguousMatch
}

// isPrefixOf reports whether prefix is a directory prefix of target.
// "a/b" is a prefix of "a/b/c" but not of "a/bc".
func isPrefixOf(prefix, target string) bool {
	// Ensure target starts with prefix + separator.
	return target == prefix || len(target) > len(prefix) && target[:len(prefix)+1] == prefix+string(os.PathSeparator)
}

// depth returns the number of path components in p.
func depth(p string) int {
	return strings.Count(p, string(os.PathSeparator)) + 1
}
