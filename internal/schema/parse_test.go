package schema

import (
	"strings"
	"testing"
	"time"
)

func TestParseIssue_synced_frontmatter(t *testing.T) {
	content := `---
key: ABC-123
type: story
project: project:ABC
schema_version: "1"
remote_version: "42"
content_hash: "abc123def"
sync_time: "2026-06-21T12:00:00Z"
---
## Description
Some description text.
`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}

	if issue.Identity.Key != "ABC-123" {
		t.Errorf("Identity.Key = %q, want %q", issue.Identity.Key, "ABC-123")
	}
	if issue.Identity.Type != "story" {
		t.Errorf("Identity.Type = %q, want %q", issue.Identity.Type, "story")
	}
	if !issue.Identity.Project.Equals(TypedRef{Type: RefProject, Value: "ABC"}) {
		t.Errorf("Identity.Project = %+v, want {Type: project, Value: ABC}", issue.Identity.Project)
	}
	if issue.MachineOwned.SchemaVersion != "1" {
		t.Errorf("MachineOwned.SchemaVersion = %q, want %q", issue.MachineOwned.SchemaVersion, "1")
	}
	if issue.RemoteMetadata.RemoteVersion != "42" {
		t.Errorf("RemoteMetadata.RemoteVersion = %q, want %q", issue.RemoteMetadata.RemoteVersion, "42")
	}
	if issue.RemoteMetadata.ContentHash != "abc123def" {
		t.Errorf("RemoteMetadata.ContentHash = %q, want %q", issue.RemoteMetadata.ContentHash, "abc123def")
	}
	wantTime, _ := time.Parse(time.RFC3339, "2026-06-21T12:00:00Z")
	if !issue.RemoteMetadata.SyncTime.Equal(wantTime) {
		t.Errorf("RemoteMetadata.SyncTime = %v, want %v", issue.RemoteMetadata.SyncTime, wantTime)
	}
}

func TestParseIssue_synced_frontmatter_only(t *testing.T) {
	content := `---
key: PROJ-456
type: bug
project: project:PROJ
schema_version: "1"
remote_version: "10"
content_hash: "xyz"
sync_time: "2026-01-01T00:00:00Z"
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}

	if issue.Identity.Key != "PROJ-456" {
		t.Errorf("Identity.Key = %q, want %q", issue.Identity.Key, "PROJ-456")
	}
	if issue.Identity.Type != "bug" {
		t.Errorf("Identity.Type = %q, want %q", issue.Identity.Type, "bug")
	}
	if !issue.Identity.Project.Equals(TypedRef{Type: RefProject, Value: "PROJ"}) {
		t.Errorf("Identity.Project = %+v, want {Type: project, Value: PROJ}", issue.Identity.Project)
	}
}

func TestParseIssue_state_file_in_synced(t *testing.T) {
	content := `---
key: XYZ-789
type: task
project: project:XYZ
schema_version: "1"
remote_version: "5"
content_hash: "hash5"
sync_time: "2026-03-15T08:30:00Z"
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}

	if issue.RemoteMetadata.State() != StateSynced {
		t.Errorf("RemoteMetadata.State() = %q, want %q", issue.RemoteMetadata.State(), StateSynced)
	}
}

func TestParseIssue_is_syncable(t *testing.T) {
	content := `---
key: SYNC-1
type: story
project: project:SYNC
schema_version: "1"
remote_version: "1"
content_hash: "abc"
sync_time: "2026-06-21T12:00:00Z"
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}

	if !issue.RemoteMetadata.IsSyncable() {
		t.Error("synced issue should be IsSyncable")
	}
}

func TestParseIssue_no_frontmatter(t *testing.T) {
	content := `Just some text without frontmatter.`

	_, err := ParseIssue(content)
	if err == nil {
		t.Fatal("ParseIssue should return error for no frontmatter")
	}
	if err.Kind != ErrKindNoFrontmatter {
		t.Errorf("ParseError.Kind = %q, want %q", err.Kind, ErrKindNoFrontmatter)
	}
	if !strings.Contains(err.Msg, "frontmatter") {
		t.Errorf("error should mention frontmatter, got: %v", err)
	}
}

func TestParseIssue_no_closing_delimiter(t *testing.T) {
	content := `---
key: ABC-1
type: story
project: project:ABC`

	_, err := ParseIssue(content)
	if err == nil {
		t.Fatal("ParseIssue should return error for no closing delimiter")
	}
	if err.Kind != ErrKindNoClosingDelimiter {
		t.Errorf("ParseError.Kind = %q, want %q", err.Kind, ErrKindNoClosingDelimiter)
	}
	if !strings.Contains(err.Msg, "closing") {
		t.Errorf("error should mention closing delimiter, got: %v", err)
	}
}

func TestParseIssue_invalid_yaml(t *testing.T) {
	content := `---
key: [invalid yaml
---`

	_, err := ParseIssue(content)
	if err == nil {
		t.Fatal("ParseIssue should return error for invalid YAML")
	}
	if err.Kind != ErrKindInvalidYAML {
		t.Errorf("ParseError.Kind = %q, want %q", err.Kind, ErrKindInvalidYAML)
	}
	if err.Msg == "" {
		t.Error("ParseError.Msg should not be empty")
	}
}

func TestParseIssue_invalid_project_ref(t *testing.T) {
	content := `---
key: ABC-1
type: story
project: invalid_no_colon
schema_version: "1"
---`

	_, err := ParseIssue(content)
	if err == nil {
		t.Fatal("ParseIssue should return error for invalid project ref")
	}
	if err.Kind != ErrKindInvalidProjectRef {
		t.Errorf("ParseError.Kind = %q, want %q", err.Kind, ErrKindInvalidProjectRef)
	}
	if err.Msg == "" {
		t.Error("ParseError.Msg should not be empty")
	}
}

func TestParseIssue_invalid_sync_time(t *testing.T) {
	content := `---
key: ABC-1
type: story
project: project:ABC
schema_version: "1"
sync_time: "not-a-date"
---`

	_, err := ParseIssue(content)
	if err == nil {
		t.Fatal("ParseIssue should return error for invalid sync_time")
	}
	if err.Kind != ErrKindInvalidSyncTime {
		t.Errorf("ParseError.Kind = %q, want %q", err.Kind, ErrKindInvalidSyncTime)
	}
	if err.Msg == "" {
		t.Error("ParseError.Msg should not be empty")
	}
	if !strings.Contains(err.Msg, "sync_time") {
		t.Errorf("error should mention sync_time, got: %v", err)
	}
}

func TestParseIssue_zero_issue(t *testing.T) {
	issue := Issue{}
	if !issue.IsZero() {
		t.Error("empty Issue should be IsZero")
	}

	issueWithIdentity := Issue{
		Identity: IssueIdentity{
			Key:     "ABC-1",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "ABC"},
		},
	}
	if issueWithIdentity.IsZero() {
		t.Error("Issue with identity should not be IsZero")
	}
}

func TestParseIssue_draft_frontmatter(t *testing.T) {
	content := `---
key: DRAFT-1
type: story
project: project:DRAFT
schema_version: "1"
state: draft
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}

	if issue.Identity.Key != "DRAFT-1" {
		t.Errorf("Identity.Key = %q, want %q", issue.Identity.Key, "DRAFT-1")
	}
	if issue.Identity.Type != "story" {
		t.Errorf("Identity.Type = %q, want %q", issue.Identity.Type, "story")
	}
	if !issue.Identity.Project.Equals(TypedRef{Type: RefProject, Value: "DRAFT"}) {
		t.Errorf("Identity.Project = %+v, want {Type: project, Value: DRAFT}", issue.Identity.Project)
	}
	if issue.MachineOwned.SchemaVersion != "1" {
		t.Errorf("MachineOwned.SchemaVersion = %q, want %q", issue.MachineOwned.SchemaVersion, "1")
	}
	if issue.RemoteMetadata.State() != StateDraft {
		t.Errorf("RemoteMetadata.State() = %q, want %q", issue.RemoteMetadata.State(), StateDraft)
	}
	if issue.RemoteMetadata.IsSyncable() {
		t.Error("draft issue should not be IsSyncable")
	}
	if !issue.RemoteMetadata.IsZero() {
		t.Error("draft issue RemoteMetadata should be IsZero (no remote fields)")
	}
}

func TestParseIssue_empty_sections(t *testing.T) {
	content := `---
key: SEC-1
type: story
project: project:SEC
schema_version: "1"
---
## Description

## Acceptance Criteria
`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}

	// B030a does not parse sections yet; Sections should be nil.
	if issue.Sections != nil {
		t.Errorf("Sections should be nil for B030a, got %v", issue.Sections)
	}
}

func TestParseErrorError(t *testing.T) {
	err := (&ParseError{
		Kind: ErrKindInvalidYAML,
		Msg:  "bad yaml",
	}).Error()
	if err != "invalid_yaml: bad yaml" {
		t.Fatalf("Error() = %q, want %q", err, "invalid_yaml: bad yaml")
	}
}
