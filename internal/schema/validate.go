package schema

import (
	"fmt"
)

// ValidationError represents a single validation failure.
type ValidationError struct {
	Field string
	Msg   string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Msg)
}

// ValidateRequired checks that all required identity fields are set.
// Returns a nil slice when the identity is complete, or a slice of errors
// for each missing or empty required field.
func (i IssueIdentity) ValidateRequired() []ValidationError {
	var errs []ValidationError
	if i.Key == "" {
		errs = append(errs, ValidationError{
			Field: "key",
			Msg:   "issue key is required",
		})
	}
	if i.Type == "" {
		errs = append(errs, ValidationError{
			Field: "type",
			Msg:   "issue type is required",
		})
	}
	if i.Project.IsZero() {
		errs = append(errs, ValidationError{
			Field: "project",
			Msg:   "project reference is required",
		})
	}
	return errs
}

// IsComplete reports whether all required identity fields are set.
func (i IssueIdentity) IsComplete() bool {
	return len(i.ValidateRequired()) == 0
}

// ValidateSections checks that every section name in the provided list is
// a known fixed section.  Unknown sections are collected and returned as
// validation errors.
func ValidateSections(sections []FixedSectionName) []ValidationError {
	var errs []ValidationError
	for _, s := range sections {
		if !s.IsKnown() {
			errs = append(errs, ValidationError{
				Field: "section",
				Msg:   fmt.Sprintf("unknown section %q", string(s)),
			})
		}
	}
	return errs
}

// Validate runs all validation checks on the issue and returns a combined
// list of errors.  An empty slice means the issue is valid.
func (i Issue) Validate() []ValidationError {
	errs := i.Identity.ValidateRequired()
	if len(i.Sections) > 0 {
		sectionNames := make([]FixedSectionName, 0, len(i.Sections))
		for name := range i.Sections {
			sectionNames = append(sectionNames, name)
		}
		errs = append(errs, ValidateSections(sectionNames)...)
	}
	errs = append(errs, i.RemoteMetadata.Validate()...)
	return errs
}

// Validate checks that remote metadata is consistent with its derived state.
// A synced issue must have RemoteVersion, ContentHash, and SyncTime set.
// A draft issue must have StateFile set to "draft".
// An archived issue must have StateFile set to "archived".
// An unsynced issue is valid when IsZero() returns true.
func (r RemoteMetadata) Validate() []ValidationError {
	switch r.State() {
	case StateSynced:
		return r.validateSynced()
	case StateDraft:
		return r.validateDraft()
	case StateArchived:
		return r.validateArchived()
	default:
		return r.validateUnsynced()
	}
}

func (r RemoteMetadata) validateSynced() []ValidationError {
	var errs []ValidationError
	if r.RemoteVersion == "" {
		errs = append(errs, ValidationError{
			Field: "remote_version",
			Msg:   "synced state requires remote_version",
		})
	}
	if r.ContentHash == "" {
		errs = append(errs, ValidationError{
			Field: "content_hash",
			Msg:   "synced state requires content_hash",
		})
	}
	if r.SyncTime.IsZero() {
		errs = append(errs, ValidationError{
			Field: "sync_time",
			Msg:   "synced state requires sync_time",
		})
	}
	return errs
}

func (r RemoteMetadata) validateDraft() []ValidationError {
	if r.StateFile != "draft" {
		return []ValidationError{{
			Field: "state",
			Msg:   "draft state requires state: draft",
		}}
	}
	return nil
}

func (r RemoteMetadata) validateArchived() []ValidationError {
	if r.StateFile != "archived" {
		return []ValidationError{{
			Field: "state",
			Msg:   "archived state requires state: archived",
		}}
	}
	return nil
}

func (r RemoteMetadata) validateUnsynced() []ValidationError {
	// Unsynced state is valid when IsZero() returns true (no remote metadata).
	// Partial metadata without a state file is also unsynced but represents
	// a partial sync that should be caught separately.
	if !r.IsZero() {
		return []ValidationError{{
			Field: "remote_metadata",
			Msg:   "unsynced state with partial metadata is inconsistent",
		}}
	}
	return nil
}
