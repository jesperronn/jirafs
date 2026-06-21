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
