package schema

// ConflictType represents the kind of conflict detected between local and
// remote state for an editable issue field.
type ConflictType string

const (
	// ConflictBothEdited means both local and remote edited the same field.
	ConflictBothEdited ConflictType = "both_edited"
	// ConflictLocalDeleteRemoteEdit means the local side deleted a field
	// that the remote side edited.
	ConflictLocalDeleteRemoteEdit ConflictType = "local_delete_remote_edit"
	// ConflictRemoteDeleteLocalEdit means the remote side deleted a field
	// that the local side edited.
	ConflictRemoteDeleteLocalEdit ConflictType = "remote_delete_local_edit"
	// ConflictLocalAddRemoteEdit means the local side added a new field
	// that the remote side also edited (remote had a prior value).
	ConflictLocalAddRemoteEdit ConflictType = "local_add_remote_edit"
	// ConflictArchivePathInvalid means the archive path configured for the
	// sync is not valid (e.g. does not exist or is not writable).
	ConflictArchivePathInvalid ConflictType = "archive_path_invalid"
	// ConflictUnresolvedRef means a reference in the issue cannot be
	// resolved (e.g. empty linked issue key, missing assignee).
	ConflictUnresolvedRef ConflictType = "unresolved_ref"
	// ConflictInvalidTransition means a requested status change is not a valid
	// transition under the current sync rules.
	ConflictInvalidTransition ConflictType = "invalid_transition"
)

// ValidConflictTypes returns the set of all recognized conflict types.
var ValidConflictTypes = []ConflictType{
	ConflictBothEdited,
	ConflictLocalDeleteRemoteEdit,
	ConflictRemoteDeleteLocalEdit,
	ConflictLocalAddRemoteEdit,
	ConflictArchivePathInvalid,
	ConflictUnresolvedRef,
	ConflictInvalidTransition,
}

// IsValidConflictType reports whether ct is a known conflict type.
func IsValidConflictType(ct ConflictType) bool {
	for _, v := range ValidConflictTypes {
		if v == ct {
			return true
		}
	}
	return false
}

// Conflict represents a detected conflict between the local and remote
// versions of an issue file for a single editable field.
//
// This model is transport-agnostic — it does not reference Jira, HTTP,
// or any sync protocol. It is a pure schema-level representation of
// what went wrong and what values are involved.
type Conflict struct {
	// Field is the editable field that conflicted.
	Field EditableField
	// Type is the kind of conflict (both_edited, local_delete_remote_edit,
	// remote_delete_local_edit, local_add_remote_edit).
	Type ConflictType
	// LocalValue is the value on the local side. Empty when the local
	// side deleted the field (for delete-type conflicts).
	LocalValue string
	// RemoteValue is the value on the remote side. Empty when the remote
	// side deleted the field (for delete-type conflicts).
	RemoteValue string
}

// IsZero reports whether c is the zero value.
func (c Conflict) IsZero() bool {
	return c.Field == "" && c.Type == "" && c.LocalValue == "" && c.RemoteValue == ""
}

// String renders the conflict as a compact "type:field:local:remote" string.
func (c Conflict) String() string {
	return string(c.Type) + ":" + string(c.Field) + ":" + c.LocalValue + ":" + c.RemoteValue
}

// Equals reports whether c and d represent the same conflict.
func (c Conflict) Equals(d Conflict) bool {
	return c.Field == d.Field &&
		c.Type == d.Type &&
		c.LocalValue == d.LocalValue &&
		c.RemoteValue == d.RemoteValue
}
