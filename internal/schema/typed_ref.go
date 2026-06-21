// Package schema defines typed reference values, issue documents, sync metadata, registry models,
// and plan/operation models.
package schema

import (
	"fmt"
	"strings"
)

// RefType represents the kind of Jira entity a typed reference points to.
type RefType string

// Supported reference types.
const (
	RefUser      RefType = "user"
	RefProject   RefType = "project"
	RefStatus    RefType = "status"
	RefSprint    RefType = "sprint"
	RefVersion   RefType = "version"
	RefEpic      RefType = "epic"
	RefIssue     RefType = "issue"
	RefIssueType RefType = "issuetype"
)

// ValidRefTypes returns the set of all recognized reference types.
var ValidRefTypes = []RefType{
	RefUser,
	RefProject,
	RefStatus,
	RefSprint,
	RefVersion,
	RefEpic,
	RefIssue,
	RefIssueType,
}

// IsValidRefType reports whether rt is a known reference type.
func IsValidRefType(rt RefType) bool {
	for _, v := range ValidRefTypes {
		if v == rt {
			return true
		}
	}
	return false
}

// TypedRef is a typed reference to a Jira entity.
// It encodes as "type:value" in issue frontmatter.
type TypedRef struct {
	Type  RefType
	Value string
}

// ParseTypedRef parses a string like "user:jesper" into a TypedRef.
// It returns an error when the format is invalid or the type is unknown.
func ParseTypedRef(s string) (TypedRef, error) {
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return TypedRef{}, fmt.Errorf("typed ref: missing type separator in %q", s)
	}
	rawType := s[:idx]
	value := s[idx+1:]
	if value == "" {
		return TypedRef{}, fmt.Errorf("typed ref: empty value in %q", s)
	}
	rt := RefType(rawType)
	if !IsValidRefType(rt) {
		return TypedRef{}, fmt.Errorf("typed ref: unknown type %q in %q", rawType, s)
	}
	return TypedRef{Type: rt, Value: value}, nil
}

// String serializes the typed reference back to "type:value" format.
func (r TypedRef) String() string {
	return fmt.Sprintf("%s:%s", r.Type, r.Value)
}

// IsZero reports whether r is the zero value.
func (r TypedRef) IsZero() bool {
	return r.Type == "" && r.Value == ""
}

// Equals reports whether r and o are equal.
func (r TypedRef) Equals(o TypedRef) bool {
	return r.Type == o.Type && r.Value == o.Value
}
