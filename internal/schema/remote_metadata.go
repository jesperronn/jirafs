package schema

import "time"

// IssueState represents the lifecycle state of a local issue file.
type IssueState string

const (
	// StateUnsynced means no remote metadata exists; the file is unsynced.
	StateUnsynced IssueState = "unsynced"
	// StateSynced means remote metadata exists; the file is in sync with Jira.
	StateSynced IssueState = "synced"
	// StateDraft means the file is a locally created draft not yet synced.
	StateDraft IssueState = "draft"
	// StateArchived means the file has been archived locally.
	StateArchived IssueState = "archived"
)

// RemoteMetadata holds fields that track synchronization state
// between a local issue file and its remote Jira counterpart.
type RemoteMetadata struct {
	RemoteVersion  string       `yaml:"remote_version"`
	ContentHash    string       `yaml:"content_hash"`
	SyncTime       time.Time    `yaml:"sync_time"`
	StateFile      string       `yaml:"state,omitempty"`
	ResolvedStatus string       `yaml:"resolved_status,omitempty"`
	Pinned         bool         `yaml:"pinned,omitempty"`
}

// IsZero reports whether r has no remote metadata set.
func (r RemoteMetadata) IsZero() bool {
	return r.RemoteVersion == "" && r.ContentHash == "" && r.SyncTime.IsZero() && r.ResolvedStatus == "" && !r.Pinned
}

// State returns the IssueState derived from r's fields.
// It returns StateDraft if StateFile is "draft", StateArchived if "archived",
// StateUnsynced if IsZero() is true, and StateSynced otherwise.
func (r RemoteMetadata) State() IssueState {
	switch r.StateFile {
	case "draft":
		return StateDraft
	case "archived":
		return StateArchived
	}
	if r.IsZero() {
		return StateUnsynced
	}
	return StateSynced
}

// IsSyncable reports whether r can participate in sync operations.
// An issue is syncable when it is in the synced state.
func (r RemoteMetadata) IsSyncable() bool {
	return r.State() == StateSynced
}
