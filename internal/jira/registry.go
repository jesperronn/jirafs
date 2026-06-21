package jira

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jirafs/jirafs/internal/registry"
	"gopkg.in/yaml.v3"
)

// RefreshRegistries fetches project-scoped registries from the Jira client
// and writes them to the mirror directory. It updates statuses, sprints,
// and fix versions in place.
//
// The function is safe to call before or after exporting an issue; it
// ensures the local registries are fresh so that typed refs can be resolved
// correctly.
func RefreshRegistries(ctx context.Context, c Client, projectKey, mirrorDir string) error {
	if err := refreshStatuses(ctx, c, mirrorDir); err != nil {
		return err
	}
	if err := refreshSprints(ctx, c, projectKey, mirrorDir); err != nil {
		return err
	}
	if err := refreshFixVersions(ctx, c, projectKey, mirrorDir); err != nil {
		return err
	}
	return nil
}

// refreshStatuses fetches all statuses from the Jira client and writes them
// to mirrorDir/statuses.yaml.
func refreshStatuses(ctx context.Context, c Client, mirrorDir string) error {
	statuses, err := c.FetchStatuses(ctx)
	if err != nil {
		return err
	}

	entries := make(map[string]registry.Status, len(statuses))
	for _, s := range statuses {
		// Use the status key as the typed-ref key when available,
		// otherwise fall back to a normalized name.
		refKey := s.StatusKey
		if refKey == "" {
			refKey = normalizeStatusRefKey(s.Name)
		}
		entries["status:"+refKey] = registry.Status{
			Name:        s.Name,
			Category:    s.Category,
			Description: s.Description,
		}
	}

	return writeRegistryFile(filepath.Join(mirrorDir, registry.StatusesFile), entries)
}

// refreshSprints fetches all sprints for the project from the Jira client
// and writes them to mirrorDir/sprints.yaml.
func refreshSprints(ctx context.Context, c Client, projectKey, mirrorDir string) error {
	sprints, err := c.FetchSprints(ctx, projectKey)
	if err != nil {
		return err
	}

	entries := make(map[string]registry.Sprint, len(sprints))
	for _, s := range sprints {
		key := fmt.Sprintf("sprint:%d", s.ID)
		sp := registry.Sprint{
			ID:           s.ID,
			Name:         s.Name,
			State:        s.State,
			CompleteDate: parseTime(s.CompleteDate),
		}
		if s.StartDate != "" {
			if t := parseTime(s.StartDate); t != nil {
				sp.StartDate = t
			}
		}
		if s.EndDate != "" {
			if t := parseTime(s.EndDate); t != nil {
				sp.EndDate = t
			}
		}
		entries[key] = sp
	}

	return writeRegistryFile(filepath.Join(mirrorDir, registry.SprintsFile), entries)
}

// refreshFixVersions fetches all fix versions for the project from the Jira
// client and writes them to mirrorDir/fix_versions.yaml.
func refreshFixVersions(ctx context.Context, c Client, projectKey, mirrorDir string) error {
	versions, err := c.FetchFixVersions(ctx, projectKey)
	if err != nil {
		return err
	}

	entries := make(map[string]registry.FixVersion, len(versions))
	for _, v := range versions {
		entries["fix-version:"+v.Name] = registry.FixVersion{
			Name:        v.Name,
			Description: v.Description,
			Archived:    v.Archived,
			Released:    v.Released,
		}
	}

	return writeRegistryFile(filepath.Join(mirrorDir, registry.FixVersionsFile), entries)
}

// writeRegistryFile writes a generic registry map to a YAML file.
// The output format matches the loader's expected shape:
//
//	entries:
//	  <key>:
//	    <fields>
func writeRegistryFile[K comparable, V any](path string, entries map[K]V) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return NewUnknownErr("cannot create registry directory: " + err.Error())
	}

	// Wrap entries in the same structure the loader unmarshals.
	wrapped := registry.RegistryFile[K, V]{Entries: entries}
	data, err := yaml.Marshal(wrapped)
	if err != nil {
		return NewUnknownErr("cannot marshal registry: " + err.Error())
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return NewUnknownErr("cannot write registry file: " + err.Error())
	}

	return nil
}

// normalizeStatusRefKey converts a status name to a valid typed-ref key
// by lowercasing and replacing spaces with hyphens.
func normalizeStatusRefKey(name string) string {
	ref := ""
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			ref += string(r + 32) // to lowercase
		} else if r == ' ' {
			ref += "-"
		} else {
			ref += string(r)
		}
	}
	return ref
}

// parseTime parses an RFC3339 date string, returning nil if empty or invalid.
func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
