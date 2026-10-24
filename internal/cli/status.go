// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

var (
	statusStdout io.Writer = os.Stdout
	statusStderr io.Writer = os.Stderr
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

	// Changes lists local issue files with unsynced edits, sorted by group
	// (conflicted, modified, draft) and then key.
	Changes []LocalChange
}

// ChangeState is the group a locally changed file belongs to.
type ChangeState string

const (
	// ChangeConflicted marks a modified file whose remote copy moved on.
	ChangeConflicted ChangeState = "conflicted"
	// ChangeModified marks a file edited since the last export/refresh/push.
	ChangeModified ChangeState = "modified"
	// ChangeDraft marks a locally created issue not yet pushed to Jira.
	ChangeDraft ChangeState = "draft"
)

// changeOrder is the stable display order of the groups.
var changeOrder = []ChangeState{ChangeConflicted, ChangeModified, ChangeDraft}

// LocalChange describes one local issue file with unsynced changes.
type LocalChange struct {
	State   ChangeState
	Key     string
	Path    string
	Command string
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
	return BuildStatusSnapshotFor(settings, cwd, "")
}

// BuildStatusSnapshotFor is BuildStatusSnapshot with an explicit project
// name; an empty project resolves from cwd or the remembered project.
func BuildStatusSnapshotFor(settings *config.Settings, cwd, project string) StatusSnapshot {
	snap := StatusSnapshot{}

	// If settings is nil, we can't resolve anything.
	if settings == nil {
		snap.MissingSteps = []string{"settings.toml not found"}
		return snap
	}

	// Resolve the project.
	resolver := context.NewResolver(settings, project)
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

	// Collect local changes (offline: local files and mirror.yml only).
	if proj, ok := settings.Projects[snap.ProjectName]; ok && snap.Resolved {
		var m mirror.Mirror
		if data, merr := readMirrorFile(filepath.Join(snap.MirrorDir, "mirror.yml")); merr == nil {
			m = *data
		}
		snap.Changes = collectLocalChanges(proj.LocalDirs, m)
	}

	// Determine missing onboarding steps.
	snap.MissingSteps = missingOnboardingSteps(settings, snap)
	snap.OnboardingComplete = len(snap.MissingSteps) == 0

	return snap
}

// collectLocalChanges scans the *.md issue files directly inside dirs and
// classifies those with unsynced edits. Files that do not parse as issues, and
// synced files with no recorded synced_hash, are skipped.
func collectLocalChanges(dirs []string, m mirror.Mirror) []LocalChange {
	var changes []LocalChange
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			doc := string(data)
			issue, pe := schema.ParseIssue(doc)
			if pe != nil {
				continue
			}
			key := string(issue.Identity.Key)
			if key == "" {
				key = strings.TrimSuffix(e.Name(), ".md")
			}
			switch {
			case issue.RemoteMetadata.State() == schema.StateDraft:
				changes = append(changes, LocalChange{ChangeDraft, key, path, "jirafs push " + path})
			case issue.RemoteMetadata.State() == schema.StateArchived:
			default:
				synced := schema.SyncedHash(doc)
				if synced == "" || synced == schema.ContentHash(doc) {
					continue
				}
				state := ChangeModified
				if remoteAhead(m.RemoteUpdatedFor(issue.Identity.Key), issue.Context.Updated) {
					state = ChangeConflicted
				}
				cmd := "jirafs push " + key
				if state == ChangeConflicted {
					cmd = "jirafs diff " + key
				}
				changes = append(changes, LocalChange{state, key, path, cmd})
			}
		}
	}
	rank := map[ChangeState]int{}
	for i, st := range changeOrder {
		rank[st] = i
	}
	sort.Slice(changes, func(i, j int) bool {
		a, b := changes[i], changes[j]
		if rank[a.State] != rank[b.State] {
			return rank[a.State] < rank[b.State]
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		return a.Path < b.Path
	})
	return changes
}

// jiraTimeLayouts are the timestamp formats Jira and jirafs write.
var jiraTimeLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05.000-0700", "2006-01-02T15:04:05-0700"}

func parseJiraTime(s string) (time.Time, bool) {
	for _, l := range jiraTimeLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// remoteAhead reports whether the stored remote updated timestamp is newer
// than the local file's updated timestamp. Either being empty means unknown.
func remoteAhead(remote, local string) bool {
	if remote == "" || local == "" {
		return false
	}
	rt, rok := parseJiraTime(remote)
	lt, lok := parseJiraTime(local)
	if rok && lok {
		return rt.After(lt)
	}
	return remote > local
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

// RunStatus dispatches the `jirafs status` subcommand. It loads the user's
// settings, builds a status snapshot, and reports config, mirror files,
// scopes, and a next-step hint.
func RunStatus(args []string) int {
	// Check for help before loading settings.
	if wantsHelpOrWord(args) {
		printStatusHelp()
		return 0
	}

	fs := quietFlagSet("status")
	porcelain := fs.Bool("porcelain", false, "print STATE<TAB>KEY<TAB>PATH lines only")
	scope := addScopeFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagError(statusStderr, "jirafs status", err)
	}

	// Load settings.
	settings, ok := loadSettingsOrHint(statusStderr, "status")
	if !ok {
		return 1
	}

	// Build the status snapshot.
	cwd, err := scope.workdir()
	if err != nil {
		fmt.Fprintf(statusStderr, "jirafs status: cannot determine working directory: %v\n", err)
		return 1
	}
	snap := BuildStatusSnapshotFor(settings, cwd, *scope.project)

	if *porcelain {
		for _, c := range snap.Changes {
			fmt.Fprintf(statusStdout, "%s\t%s\t%s\n", c.State, c.Key, c.Path)
		}
		return 0
	}

	// Report config.
	fmt.Fprintln(statusStdout, "jirafs status:")
	fmt.Fprintln(statusStdout)

	// Project info.
	fmt.Fprintln(statusStdout, "Config:")
	fmt.Fprintf(statusStdout, "  project: %s\n", snap.ProjectName)
	fmt.Fprintf(statusStdout, "  key:     %s\n", snap.ProjectKey)
	fmt.Fprintf(statusStdout, "  instance: %s\n", snap.Instance)
	fmt.Fprintf(statusStdout, "  resolved: %v\n", snap.Resolved)
	fmt.Fprintln(statusStdout)

	// Mirror info.
	fmt.Fprintln(statusStdout, "Mirror:")
	fmt.Fprintf(statusStdout, "  dir: %s\n", snap.MirrorDir)
	fmt.Fprintf(statusStdout, "  exists: %v\n", snap.MirrorExists)
	if snap.MirrorExists {
		fmt.Fprintf(statusStdout, "  scopes:   %d\n", len(snap.MirrorScopes))
		fmt.Fprintf(statusStdout, "  explicitly imported: %d\n", snap.MirrorIssueCount)
		fmt.Fprintf(statusStdout, "  scope members: %d\n", snap.MirrorScopeMemberCount)
		if len(snap.MirrorScopes) > 0 {
			fmt.Fprintln(statusStdout, "  scope names:")
			sortedScopes := make([]string, len(snap.MirrorScopes))
			copy(sortedScopes, snap.MirrorScopes)
			sort.Strings(sortedScopes)
			for _, name := range sortedScopes {
				fmt.Fprintf(statusStdout, "    - %s\n", name)
			}
		}
	}
	fmt.Fprintln(statusStdout)

	// Local changes.
	fmt.Fprintln(statusStdout, "Local changes:")
	if len(snap.Changes) == 0 {
		fmt.Fprintln(statusStdout, "  no local changes")
	}
	for _, st := range changeOrder {
		var group []LocalChange
		for _, c := range snap.Changes {
			if c.State == st {
				group = append(group, c)
			}
		}
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(statusStdout, "  %s:\n", st)
		for _, c := range group {
			fmt.Fprintf(statusStdout, "    %s  %s\n      run: %s\n", c.Key, c.Path, c.Command)
		}
	}
	fmt.Fprintln(statusStdout)

	// Onboarding / next-step hint.
	fmt.Fprintln(statusStdout, "Onboarding:")
	fmt.Fprintf(statusStdout, "  complete: %v\n", snap.OnboardingComplete)
	if len(snap.MissingSteps) > 0 {
		fmt.Fprintln(statusStdout, "  missing steps:")
		for _, step := range snap.MissingSteps {
			fmt.Fprintf(statusStdout, "    - %s\n", step)
		}
		fmt.Fprintf(statusStdout, "  next step: %s\n", snap.NextStep())
	} else {
		fmt.Fprintln(statusStdout, "  next step: (none — all setup complete)")
	}

	return 0
}

// printStatusHelp prints usage information for the status subcommand.
func printStatusHelp() {
	PrintCommandHelp(statusStdout, "status")
}
