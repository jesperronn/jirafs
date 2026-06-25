package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Instance holds one Jira site definition.
type Instance struct {
	BaseURL        string   `toml:"base_url"`
	AuthType       string   `toml:"auth_type"`
	CredentialRefs []string `toml:"credential_refs"`
}

// State holds remembered operator context.
type State struct {
	CurrentProject string `toml:"current_project"`
	CurrentUser    string `toml:"current_user"`
}

// Project holds one project definition.
type Project struct {
	Key         string   `toml:"key"`
	Instance    string   `toml:"instance"`
	MirrorDir   string   `toml:"mirror_dir"`
	LocalDirs   []string `toml:"local_dirs"`
	DefaultUser string   `toml:"default_user"`
}

// Settings is the top-level parsed settings document.
type Settings struct {
	Version   int                 `toml:"version"`
	Instances map[string]Instance `toml:"-"`
	Projects  map[string]Project  `toml:"-"`
	State     State               `toml:"state"`
}

// SettingsTOML is the raw TOML mapping used during unmarshalling.
type SettingsTOML struct {
	Version   int                 `toml:"version"`
	Instances map[string]Instance `toml:"instances"`
	Projects  map[string]Project  `toml:"projects"`
	State     State               `toml:"state"`
}

const (
	// settingsDir is the dot-directory under $HOME where jirafs stores settings.
	settingsDir = ".jirafs"

	// settingsFile is the TOML file inside settingsDir.
	settingsFile = "settings.toml"
)

// Load reads ~/.jirafs/settings.toml, parses it, validates it, and expands
// all paths. It returns a fully populated Settings on success.
func Load() (*Settings, error) {
	s, err := loadSettings()
	if err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	if err := s.expandPaths(); err != nil {
		return nil, err
	}
	return s, nil
}

// loadSettings reads and decodes the settings TOML file.
func loadSettings() (*Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, NewSettingError(ErrMissingField, "home directory is not set", "home", "")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewSettingError(ErrMissingField, "settings file not found: "+path, "path", path)
		}
		return nil, NewSettingError(ErrMissingField, "cannot read settings file: "+err.Error(), "path", path)
	}

	var raw SettingsTOML
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, NewSettingError(ErrMissingField, "invalid TOML: "+err.Error(), "file", path)
	}

	s := &Settings{
		Version:   raw.Version,
		Instances: raw.Instances,
		Projects:  raw.Projects,
		State:     raw.State,
	}
	return s, nil
}

// settingsPath returns the absolute path to ~/.jirafs/settings.toml.
func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, settingsDir, settingsFile), nil
}

func ensureSettingsDirExists() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return NewSettingError(ErrMissingField, "home directory is not set", "home", "")
	}
	dir := filepath.Join(home, settingsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return NewSettingError(ErrMissingField, "cannot create settings directory: "+err.Error(), "path", dir)
	}
	return nil
}

// validate checks the parsed settings for structural correctness.
func (s *Settings) validate() error {
	if s.Version != 1 {
		return NewSettingError(ErrMissingField, "only version 1 is supported, got "+string(rune('0'+s.Version)), "version", "")
	}

	// Validate instances first so URL errors surface before project errors.
	for name, inst := range s.Instances {
		if inst.BaseURL == "" {
			return NewSettingError(ErrMissingField, "base_url is required", "instances."+name+".base_url", "")
		}
		if !strings.HasPrefix(inst.BaseURL, "http://") && !strings.HasPrefix(inst.BaseURL, "https://") {
			return NewSettingError(ErrInvalidURL, "must be an absolute URL starting with http:// or https://", "instances."+name+".base_url", inst.BaseURL)
		}
		if inst.AuthType == "" {
			return NewSettingError(ErrMissingField, "auth_type is required", "instances."+name+".auth_type", "")
		}
	}

	if len(s.Instances) == 0 {
		return NewSettingError(ErrMissingField, "at least one instance is required", "instances", "")
	}

	// Validate projects.
	for name, proj := range s.Projects {
		if proj.Key == "" {
			return NewSettingError(ErrMissingField, "key is required", "projects."+name+".key", "")
		}
		if proj.Instance == "" {
			return NewSettingError(ErrMissingField, "instance is required", "projects."+name+".instance", "")
		}
		if _, ok := s.Instances[proj.Instance]; !ok {
			return NewSettingError(ErrUnknownInstance, "instance %q not found", "projects."+name+".instance", proj.Instance)
		}
		if proj.MirrorDir == "" {
			return NewSettingError(ErrMissingField, "mirror_dir is required", "projects."+name+".mirror_dir", "")
		}
	}

	if len(s.Projects) == 0 {
		return NewSettingError(ErrMissingField, "at least one project is required", "projects", "")
	}

	// Check for duplicate project keys.
	keyMap := make(map[string]string, len(s.Projects))
	for name, proj := range s.Projects {
		if proj.Key == "" {
			continue // already caught above
		}
		if prev, ok := keyMap[proj.Key]; ok {
			return NewSettingError(ErrDuplicateProjectKey,
				fmt.Sprintf("project key %q is duplicate: %q and %q", proj.Key, prev, name),
				"projects.", proj.Key)
		}
		keyMap[proj.Key] = name
	}

	// Check for duplicate mirror_dirs across projects.
	mirrorMap := make(map[string]string, len(s.Projects))
	for name, proj := range s.Projects {
		if proj.MirrorDir == "" {
			continue // already caught above
		}
		if prev, ok := mirrorMap[proj.MirrorDir]; ok {
			return NewSettingError(ErrDuplicateMirrorDir,
				fmt.Sprintf("mirror_dir %q is duplicate: %q and %q", proj.MirrorDir, prev, name),
				"projects.", proj.MirrorDir)
		}
		mirrorMap[proj.MirrorDir] = name
	}

	// Check for duplicate local_dirs across projects.
	localDirMap := make(map[string]string, len(s.Projects))
	for name, proj := range s.Projects {
		for _, ld := range proj.LocalDirs {
			if ld == "" {
				continue
			}
			if prev, ok := localDirMap[ld]; ok {
				return NewSettingError(ErrDuplicateLocalDir,
					fmt.Sprintf("local_dir %q is duplicate: %q and %q", ld, prev, name),
					"projects.", ld)
			}
			localDirMap[ld] = name
		}
	}

	// Validate state references.
	if s.State.CurrentProject != "" {
		if _, ok := s.Projects[s.State.CurrentProject]; !ok {
			return NewSettingError(ErrUnknownProject, "project %q not found", "state.current_project", s.State.CurrentProject)
		}
	}

	return nil
}

// expandPaths expands tilde and environment variable references in all paths.
func (s *Settings) expandPaths() error {
	for name, proj := range s.Projects {
		// Expand mirror_dir.
		expanded, err := expandPath(proj.MirrorDir)
		if err != nil {
			return NewSettingError(ErrMissingField, "cannot expand mirror_dir: "+err.Error(),
				"projects."+name+".mirror_dir", proj.MirrorDir)
		}
		if expanded == "" {
			return NewSettingError(ErrEmptyMirrorDir, "mirror_dir must not be empty after expansion",
				"projects."+name+".mirror_dir", proj.MirrorDir)
		}

		// Expand local_dirs.
		expandedDirs := make([]string, 0, len(proj.LocalDirs))
		for _, d := range proj.LocalDirs {
			ed, err := expandPath(d)
			if err != nil {
				return NewSettingError(ErrMissingField, "cannot expand local_dir: "+err.Error(),
					"projects."+name+".local_dirs", d)
			}
			if ed != "" {
				expandedDirs = append(expandedDirs, ed)
			}
		}

		// Write back the updated project.
		s.Projects[name] = Project{
			Key:         proj.Key,
			Instance:    proj.Instance,
			MirrorDir:   expanded,
			LocalDirs:   expandedDirs,
			DefaultUser: proj.DefaultUser,
		}
	}
	return nil
}

// SaveState writes the current State back to the settings file.
// It preserves existing instances and projects by loading the file first,
// updating only the state, and writing the full file.
func (s *Settings) SaveState() error {
	path, err := settingsPath()
	if err != nil {
		return NewSettingError(ErrMissingField, "home directory is not set", "home", "")
	}

	// Load existing settings to preserve instances and projects.
	existing, err := s.loadOrCreate()
	if err != nil {
		return err
	}

	// Update only the state.
	existing.State = s.State

	// Write the full settings file using SettingsTOML which has proper TOML
	// tags for Instances and Projects.
	toM := SettingsTOML{
		Version:   existing.Version,
		Instances: existing.Instances,
		Projects:  existing.Projects,
		State:     existing.State,
	}

	data, err := toml.Marshal(toM)
	if err != nil {
		return NewSettingError(ErrMissingField, "cannot marshal settings: "+err.Error(), "state", "")
	}

	if err := ensureSettingsDirExists(); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return NewSettingError(ErrMissingField, "cannot write settings file: "+err.Error(), "path", path)
	}

	return nil
}

// SetupProject records one instance and one project in the settings file.
// It creates the settings file if it does not exist, or merges into an
// existing file. The instance is keyed by instanceName, the project by
// projectName. The caller provides projectKey, baseURL, authType, mirrorDir,
// and an optional set of credential ref strings.
//
// If the instance already exists, base_url and auth_type are overwritten
// and credential_refs are appended (duplicates are not deduplicated).
// If the project already exists, key, instance, and mirror_dir are
// overwritten. LocalDirs are not touched.
//
// After mutation the settings are validated and paths are expanded.
// A validation failure leaves the file unchanged.
func (s *Settings) SetupProject(
	instanceName string,
	projectName string,
	projectKey string,
	baseURL string,
	authType string,
	mirrorDir string,
	credentialRefs []string,
) error {
	// Load existing settings, or start fresh.
	existing, err := s.loadOrCreate()
	if err != nil {
		return err
	}

	// Save a snapshot for rollback on validation failure.
	snapshot := *existing

	// Ensure version is set.
	if existing.Version == 0 {
		existing.Version = 1
	}

	// Initialize maps if nil.
	if existing.Instances == nil {
		existing.Instances = make(map[string]Instance)
	}
	if existing.Projects == nil {
		existing.Projects = make(map[string]Project)
	}

	// Upsert the instance.
	inst, ok := existing.Instances[instanceName]
	if !ok {
		inst = Instance{}
	}
	inst.BaseURL = baseURL
	inst.AuthType = authType
	inst.CredentialRefs = append(inst.CredentialRefs, credentialRefs...)
	existing.Instances[instanceName] = inst

	// Upsert the project.
	proj, ok := existing.Projects[projectName]
	if !ok {
		proj = Project{}
	}
	proj.Key = projectKey
	proj.Instance = instanceName
	proj.MirrorDir = mirrorDir
	if len(proj.LocalDirs) == 0 {
		proj.LocalDirs = []string{mirrorDir}
	}
	existing.Projects[projectName] = proj

	// Validate before persisting.
	if err := existing.validate(); err != nil {
		// Rollback: do not write the file.
		_ = snapshot
		return err
	}

	// Expand paths.
	if err := existing.expandPaths(); err != nil {
		return err
	}

	// Write the full settings file (not just state).
	path, err := settingsPath()
	if err != nil {
		return NewSettingError(ErrMissingField, "home directory is not set", "home", "")
	}

	// Marshal using SettingsTOML which has proper TOML tags for
	// Instances and Projects.
	toM := SettingsTOML{
		Version:   existing.Version,
		Instances: existing.Instances,
		Projects:  existing.Projects,
		State:     existing.State,
	}

	data, err := toml.Marshal(toM)
	if err != nil {
		return NewSettingError(ErrMissingField, "cannot marshal settings: "+err.Error(), "state", "")
	}

	if err := ensureSettingsDirExists(); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return NewSettingError(ErrMissingField, "cannot write settings file: "+err.Error(), "path", path)
	}

	return nil
}

// loadOrCreate reads the settings file, or returns a zero-value Settings
// when the file does not exist (no error). This is the internal variant
// that skips validation and path expansion — callers must do that themselves.
func (s *Settings) loadOrCreate() (*Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, NewSettingError(ErrMissingField, "home directory is not set", "home", "")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No file yet: return a zero-value Settings.
			return &Settings{
				Version:   1,
				Instances: make(map[string]Instance),
				Projects:  make(map[string]Project),
			}, nil
		}
		return nil, NewSettingError(ErrMissingField, "cannot read settings file: "+err.Error(), "path", path)
	}

	var raw SettingsTOML
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, NewSettingError(ErrMissingField, "invalid TOML: "+err.Error(), "file", path)
	}

	return &Settings{
		Version:   raw.Version,
		Instances: raw.Instances,
		Projects:  raw.Projects,
		State:     raw.State,
	}, nil
}

// expandPath expands ~ and $VAR references in a single path string.
func expandPath(p string) (string, error) {
	if p == "" {
		return "", nil
	}

	// Expand ~ at the start.
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}

	// Expand environment variables like $VAR or ${VAR}.
	expanded := os.Expand(p, func(varName string) string {
		return os.Getenv(varName)
	})

	// If no expansion happened (no $VAR found), return as-is.
	if expanded == p {
		return p, nil
	}

	return expanded, nil
}
