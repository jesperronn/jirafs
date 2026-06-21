// Package archive provides the archive service interface for moving
// archive-eligible issues to the archive directory.
package archive

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
