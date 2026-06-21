package registry

import (
	"errors"
	"fmt"
	"strings"
)

// ResolveError wraps a structured error for typed-ref resolution failures.
type ResolveError struct {
	RefType string
	Ref     string
	Code    string
	Msg     string
}

func (e *ResolveError) Error() string {
	return "registry: " + e.Code + ": " + e.Msg
}

// IsResolveError returns true if err is a *ResolveError with the given code.
func IsResolveError(err error, code string) bool {
	if err == nil {
		return false
	}
	for {
		if re, ok := err.(*ResolveError); ok {
			return re.Code == code
		}
		if uw, ok := err.(interface{ Unwrap() error }); ok && uw.Unwrap() != nil {
			err = uw.Unwrap()
			continue
		}
		return false
	}
}

// ErrMissingRef returns a *ResolveError for a missing typed ref.
// The error includes the ref type and the lookup value so callers
// can report exactly what was searched for.
func ErrMissingRef(refType, lookupValue string) *ResolveError {
	return &ResolveError{
		RefType: refType,
		Ref:     lookupValue,
		Code:    "missing_ref",
		Msg:     fmt.Sprintf("no %s found for %s", refType, lookupValue),
	}
}

var errMissingRef = errors.New("registry: missing_ref: typed ref is empty")

// ParseTypedRef splits a typed ref like "user:jesper" into (type, key).
// Returns errMissingRef when ref is empty or has no colon separator.
func ParseTypedRef(ref string) (refType, key string, err error) {
	if ref == "" {
		return "", "", errMissingRef
	}
	idx := strings.IndexByte(ref, ':')
	if idx < 0 {
		return "", "", errMissingRef
	}
	return ref[:idx], ref[idx+1:], nil
}

// ResolveUser looks up a user typed ref in the users registry and returns
// the Jira account_id. Returns ("", false) when the ref is not found.
//
// Typed ref format: "user:<account_id>"
//
// Example: "user:712020:abcd" → "712020:abcd"
func ResolveUser(typedRef string, users map[string]User) (accountID string, found bool) {
	if typedRef == "" {
		return "", false
	}
	u, ok := users[typedRef]
	if !ok {
		return "", false
	}
	return u.AccountID, true
}

// ResolveProject looks up a project typed ref in the projects registry and
// returns the Jira project key. Returns ("", false) when the ref is not found.
//
// Typed ref format: "project:<key>"
//
// Example: "project:ABC" → "ABC"
func ResolveProject(typedRef string, projects map[string]Project) (key string, found bool) {
	if typedRef == "" {
		return "", false
	}
	p, ok := projects[typedRef]
	if !ok {
		return "", false
	}
	return p.Key, true
}

// ResolveStatus looks up a status typed ref in the statuses registry and
// returns the status name. Returns ("", false) when the ref is not found.
//
// Typed ref format: "status:<key>"
//
// Example: "status:in-progress" → "In Progress"
func ResolveStatus(typedRef string, statuses map[string]Status) (name string, found bool) {
	if typedRef == "" {
		return "", false
	}
	s, ok := statuses[typedRef]
	if !ok {
		return "", false
	}
	return s.Name, true
}

// ResolveSprint looks up a sprint typed ref in the sprints registry and
// returns the Jira-assigned sprint ID as a string. Returns ("", false) when
// the ref is not found.
//
// Typed ref format: "sprint:<id>"
//
// Example: "sprint:12345" → "12345"
func ResolveSprint(typedRef string, sprints map[string]Sprint) (id string, found bool) {
	if typedRef == "" {
		return "", false
	}
	s, ok := sprints[typedRef]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%d", s.ID), true
}

// ResolveFixVersion looks up a fix-version typed ref in the fix-version
// registry and returns the version name. Returns ("", false) when the ref
// is not found.
//
// Typed ref format: "fix-version:<name>"
//
// Example: "fix-version:1.4.0" → "1.4.0"
func ResolveFixVersion(typedRef string, fixVersions map[string]FixVersion) (name string, found bool) {
	if typedRef == "" {
		return "", false
	}
	fv, ok := fixVersions[typedRef]
	if !ok {
		return "", false
	}
	return fv.Name, true
}
