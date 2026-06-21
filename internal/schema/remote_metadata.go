package schema

import "time"

// RemoteMetadata holds fields that track synchronization state
// between a local issue file and its remote Jira counterpart.
type RemoteMetadata struct {
	RemoteVersion string       `yaml:"remote_version"`
	ContentHash   string       `yaml:"content_hash"`
	SyncTime      time.Time    `yaml:"sync_time"`
}

// IsZero reports whether r has no remote metadata set.
func (r RemoteMetadata) IsZero() bool {
	return r.RemoteVersion == "" && r.ContentHash == "" && r.SyncTime.IsZero()
}
