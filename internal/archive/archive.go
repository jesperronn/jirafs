// Package archive provides the archive service interface and filesystem-backed
// movement for archive-eligible issue snapshots.
package archive

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

// Service defines the interface for archive operations. Implementations
// handle the actual file movement and metadata updates.
type Service interface {
	// Archive moves an issue file to the archive directory.
	// The eligible parameter describes the issue and its resolved status.
	// The mirrorDir is the project's mirror directory containing the mirror file.
	// The localDir is the project's local directory where the issue file lives.
	// The issuePath is the absolute path to the issue file.
	Archive(eligible string, mirrorDir, localDir, issuePath string) error
}

// ServiceFunc is an adapter to allow the use of ordinary functions as Service.
type ServiceFunc func(eligible string, mirrorDir, localDir, issuePath string) error

// Archive implements Service.
func (f ServiceFunc) Archive(eligible string, mirrorDir, localDir, issuePath string) error {
	return f(eligible, mirrorDir, localDir, issuePath)
}

// FileService archives issue snapshots into a dedicated archive directory.
type FileService struct {
	ArchiveDir string
}

// Archive rewrites the issue into archived state, writes it into the archive
// directory, and removes the live copy only after the snapshot write succeeds.
// It respects live membership rules: issues that are pinned, unsynced,
// explicitly imported into the mirror, or scope members are not archived.
func (s FileService) Archive(eligible string, mirrorDir, localDir, issuePath string) error {
	if s.ArchiveDir == "" {
		return fmt.Errorf("archive directory is empty")
	}
	if err := os.MkdirAll(s.ArchiveDir, 0o755); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	data, err := os.ReadFile(issuePath)
	if err != nil {
		return fmt.Errorf("read issue file: %w", err)
	}

	issue, parseErr := schema.ParseIssue(string(data))
	if parseErr != nil {
		return fmt.Errorf("parse issue file: %w", parseErr)
	}

	// Check live membership rules before archiving.
	m, _, loadErr := loadMirrorYAML(mirrorDir)
	if loadErr != nil {
		return fmt.Errorf("load mirror file: %w", loadErr)
	}

	if !m.IsArchiveEligible(issue.Identity.Key, mirror.ResolvedStatus(issue.RemoteMetadata.ResolvedStatus), issue.RemoteMetadata) {
		return fmt.Errorf("issue %s is not archive-eligible: live membership rules prevent archiving", eligible)
	}

	issue.RemoteMetadata.StateFile = string(schema.StateArchived)
	snapshot := schema.RenderIssue(issue)
	dest := filepath.Join(s.ArchiveDir, filepath.Base(issuePath))
	if err := os.WriteFile(dest, []byte(snapshot), 0o644); err != nil {
		return fmt.Errorf("write archived file: %w", err)
	}
	if err := os.Remove(issuePath); err != nil {
		return fmt.Errorf("remove original file: %w", err)
	}
	return nil
}

// loadMirrorYAML reads the mirror file from the mirror directory.
// It looks for mirror.yml or mirror.yaml in the mirror directory.
// If no mirror file is found, it returns an empty mirror (all issues are eligible).
func loadMirrorYAML(mirrorDir string) (*mirror.Mirror, string, error) {
	for _, name := range []string{"mirror.yml", "mirror.yaml"} {
		path := filepath.Join(mirrorDir, name)
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, "", fmt.Errorf("cannot read mirror file %s: %w", path, err)
			}
			m, err := mirror.UnmarshalMirror(data)
			if err != nil {
				return nil, "", fmt.Errorf("cannot parse mirror file %s: %w", path, err)
			}
			return m, path, nil
		}
	}
	// No mirror file found: return an empty mirror (all issues are eligible).
	return &mirror.Mirror{}, filepath.Join(mirrorDir, "mirror.yml"), nil
}
