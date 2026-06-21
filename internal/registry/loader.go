package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Registry file names.
const (
	UsersFile      = "users.yaml"
	ProjectsFile   = "projects.yaml"
	StatusesFile   = "statuses.yaml"
	SprintsFile    = "sprints.yaml"
	FixVersionsFile = "fix_versions.yaml"
	IssueTypesFile = "issue_types.yaml"
)

// RegistryError wraps a structured error for registry file loading failures.
type RegistryError struct {
	File  string
	Code  string
	Msg   string
}

func (e *RegistryError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("registry: %s (%s): %s", e.Code, e.File, e.Msg)
	}
	return fmt.Sprintf("registry: %s: %s", e.Code, e.Msg)
}

// IsRegistryError returns true if err is a *RegistryError with the given code.
func IsRegistryError(err error, code string) bool {
	if err == nil {
		return false
	}
	for {
		if se, ok := err.(*RegistryError); ok {
			return se.Code == code
		}
		if uw, ok := err.(interface{ Unwrap() error }); ok && uw.Unwrap() != nil {
			err = uw.Unwrap()
			continue
		}
		return false
	}
}

// NewRegistryError creates a RegistryError.
func NewRegistryError(file, code, msg string) *RegistryError {
	return &RegistryError{
		File: file,
		Code: code,
		Msg:  msg,
	}
}

// RegistryFile represents any registry YAML file that has a top-level map
// keyed by typed-ref strings.
type RegistryFile[K comparable, V any] struct {
	Entries map[K]V `yaml:"entries"`
}

// LoadUsersFile reads a users registry YAML from the given path and returns
// a map keyed by typed-ref (e.g. "user:jesper") to User.
//
// Expected file shape:
//
//	entries:
//	  "user:jesper":
//	    account_id: "712020:abcd"
//	    display_name: "Jesper Ronn"
//	    email: "jesper@example.com"
//	    active: true
func LoadUsersFile(path string) (map[string]User, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewRegistryError(path, "file_not_found", "users registry file not found")
		}
		return nil, NewRegistryError(path, "file_read", "cannot read users registry file: "+err.Error())
	}

	var raw RegistryFile[string, User]
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, NewRegistryError(path, "yaml_parse", "invalid users registry YAML: "+err.Error())
	}

	if raw.Entries == nil {
		return nil, NewRegistryError(path, "missing_entries", "users registry file has no entries key")
	}

	return raw.Entries, nil
}

// LoadProjectsFile reads a projects registry YAML from the given path and returns
// a map keyed by typed-ref (e.g. "project:abc") to Project.
//
// Expected file shape:
//
//	entries:
//	  "project:abc":
//	    key: "ABC"
//	    name: "A Big Project"
//	    id: "10000"
func LoadProjectsFile(path string) (map[string]Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewRegistryError(path, "file_not_found", "projects registry file not found")
		}
		return nil, NewRegistryError(path, "file_read", "cannot read projects registry file: "+err.Error())
	}

	var raw RegistryFile[string, Project]
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, NewRegistryError(path, "yaml_parse", "invalid projects registry YAML: "+err.Error())
	}

	if raw.Entries == nil {
		return nil, NewRegistryError(path, "missing_entries", "projects registry file has no entries key")
	}

	return raw.Entries, nil
}

// LoadUsers loads the users registry from mirrorDir/users.yaml.
func LoadUsers(mirrorDir string) (map[string]User, error) {
	return LoadUsersFile(filepath.Join(mirrorDir, UsersFile))
}

// LoadProjects loads the projects registry from mirrorDir/projects.yaml.
func LoadProjects(mirrorDir string) (map[string]Project, error) {
	return LoadProjectsFile(filepath.Join(mirrorDir, ProjectsFile))
}
