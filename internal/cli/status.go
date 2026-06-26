// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

// StatusSnapshot represents the current status of a project, mirror, and
// onboarding state. It is consumed by the status and doctor commands to
// report the current state of a jirafs workspace.
type StatusSnapshot struct {
	// ProjectName is the config name of the resolved project.
	ProjectName string

	// ProjectKey is the Jira project key (e.g. "PROJ").
	ProjectKey string

	// MirrorDir is the expanded mirror directory path.
	MirrorDir string

	// Instance is the Jira instance name.
	Instance string

	// Resolved reports whether a project was successfully resolved.
	Resolved bool

	// MirrorExists reports whether a mirror.yml was found in the mirror dir.
	MirrorExists bool

	// MirrorScopes lists the names of scopes defined in the mirror.
	MirrorScopes []string

	// MirrorIssueCount is the number of explicitly imported issues.
	MirrorIssueCount int

	// MirrorScopeMemberCount is the number of scope-bound issues.
	MirrorScopeMemberCount int

	// OnboardingComplete reports whether all required setup steps are done.
	OnboardingComplete bool

	// MissingSteps lists the setup steps that are still needed.
	MissingSteps []string
}

// IsZero reports whether s is the zero value.
func (s StatusSnapshot) IsZero() bool {
	return s.ProjectName == "" && s.ProjectKey == "" &&
		s.MirrorDir == "" && s.Instance == "" && !s.Resolved &&
		!s.MirrorExists && len(s.MissingSteps) == 0
}

// BuildStatusSnapshot builds a status snapshot for the given settings and
// working directory. It resolves the project, inspects the mirror file,
// and checks which onboarding steps are complete.
func BuildStatusSnapshot(settings *config.Settings, cwd string) StatusSnapshot {
	snap := StatusSnapshot{}

	// If settings is nil, we can't resolve anything.
	if settings == nil {
		snap.MissingSteps = []string{"settings.toml not found"}
		return snap
	}

	// Resolve the project.
	resolver := context.NewResolver(settings, "")
	ctx, err := resolver.Resolve(cwd)
	if err == nil {
		snap.Resolved = true
		snap.ProjectName = ctx.Name
		snap.ProjectKey = ctx.Key
		snap.MirrorDir = ctx.MirrorDir
		snap.Instance = ctx.Instance
	}

	// Check mirror file.
	if snap.MirrorDir != "" {
		mirrorPath := filepath.Join(snap.MirrorDir, "mirror.yml")
		if data, merr := readMirrorFile(mirrorPath); merr == nil {
			snap.MirrorExists = true
			m := *data
			snap.MirrorScopes = scopeNames(m)
			snap.MirrorIssueCount = len(m.Issues)
			snap.MirrorScopeMemberCount = len(m.ScopeMembers)
		}
	}

	// Determine missing onboarding steps.
	snap.MissingSteps = missingOnboardingSteps(settings, snap)
	snap.OnboardingComplete = len(snap.MissingSteps) == 0

	return snap
}

// readMirrorFile reads and parses a mirror.yml file from the given path.
func readMirrorFile(path string) (*mirror.Mirror, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return mirror.UnmarshalMirror(data)
}

// scopeNames returns the names of all scopes in the mirror.
func scopeNames(m mirror.Mirror) []string {
	names := make([]string, 0, len(m.Scopes))
	for _, s := range m.Scopes {
		names = append(names, s.Name)
	}
	return names
}

// missingOnboardingSteps returns the list of setup steps that are still
// needed for a workspace to be fully operational. A step is missing when
// its condition is not met.
func missingOnboardingSteps(settings *config.Settings, snap StatusSnapshot) []string {
	var missing []string

	// Step 1: Settings file exists.
	if settings == nil {
		missing = append(missing, "settings.toml not found")
		return missing
	}

	// Step 2: At least one instance is configured.
	if len(settings.Instances) == 0 {
		missing = append(missing, "no Jira instance configured")
	}

	// Step 3: At least one project is configured.
	if len(settings.Projects) == 0 {
		missing = append(missing, "no project configured")
	}

	// Step 4: Instance credentials are resolvable (live-probe).
	if snap.Instance != "" {
		inst, ok := settings.Instances[snap.Instance]
		if ok && len(inst.CredentialRefs) == 0 {
			missing = append(missing, "no credentials for instance %q", snap.Instance)
		}
	}

	// Step 5: Mirror directory exists and contains mirror.yml.
	if snap.MirrorDir != "" && !snap.MirrorExists {
		missing = append(missing, "mirror.yml not found in mirror directory")
	}

	// Step 6: At least one scope is defined (for full mirror functionality).
	if snap.MirrorExists && len(snap.MirrorScopes) == 0 {
		missing = append(missing, "no scopes defined in mirror")
	}

	// Step 7: At least one issue is imported or in scope.
	if snap.MirrorExists && snap.MirrorIssueCount == 0 && snap.MirrorScopeMemberCount == 0 {
		missing = append(missing, "no issues imported or in scope")
	}

	// Step 8: Project is resolved for the current directory.
	if !snap.Resolved {
		missing = append(missing, "project not resolved for current directory")
	}

	return missing
}

// HasStep reports whether the snapshot has the given step in its missing list.
func (s StatusSnapshot) HasStep(step string) bool {
	for _, m := range s.MissingSteps {
		if strings.Contains(m, step) {
			return true
		}
	}
	return false
}

// NextStep returns the first missing step, or an empty string if complete.
func (s StatusSnapshot) NextStep() string {
	if len(s.MissingSteps) == 0 {
		return ""
	}
	return s.MissingSteps[0]
}

// IssueKey is a type alias for issue keys used in status reporting.
type IssueKey = schema.IssueKey
