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

	// B031b populates Sections for every issue with a body.
	if issue.Sections == nil {
		t.Fatal("Sections should be populated, got nil")
	}
	// Both Description and Acceptance Criteria must exist, even when empty.
	if issue.Sections[SecDescription] != "" {
		t.Errorf("Sections[Description] = %q, want %q", issue.Sections[SecDescription], "")
	}
	if issue.Sections[SecAcceptanceCriteria] != "" {
		t.Errorf("Sections[Acceptance Criteria] = %q, want %q", issue.Sections[SecAcceptanceCriteria], "")
	}
}

func TestExtractFrontmatter_returnsBodyAfterClosingDelimiter(t *testing.T) {
	content := `---
key: ABC-123
type: story
project: project:ABC
---
## Description
Line one.
`

	frontmatter, body, err := extractFrontmatter(content)
	if err != nil {
		t.Fatalf("extractFrontmatter returned error: %v", err)
	}

	if !strings.Contains(frontmatter, "key: ABC-123") {
		t.Fatalf("frontmatter = %q, want key field", frontmatter)
	}
	if body != "## Description\nLine one." {
		t.Fatalf("body = %q, want %q", body, "## Description\nLine one.")
	}
}

func TestSplitSectionBlocks_preservesOrderedH2Blocks(t *testing.T) {
	body := `## Description
First line.
Still description.

## Acceptance Criteria
- one
- two

## Notes
Trailing note.`

	got := splitSectionBlocks(body)
	want := []sectionBlock{
		{
			Heading: "Description",
			Body:    "First line.\nStill description.",
		},
		{
			Heading: "Acceptance Criteria",
			Body:    "- one\n- two",
		},
		{
			Heading: "Notes",
			Body:    "Trailing note.",
		},
	}

	if len(got) != len(want) {
		t.Fatalf("len(section blocks) = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("section block %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSplitSectionBlocks_keepsEmptySectionBodies(t *testing.T) {
	body := `## Description

## Acceptance Criteria
`

	got := splitSectionBlocks(body)
	if len(got) != 2 {
		t.Fatalf("len(section blocks) = %d, want 2", len(got))
	}
	if got[0].Heading != "Description" || got[0].Body != "" {
		t.Fatalf("first section = %+v, want empty Description body", got[0])
	}
	if got[1].Heading != "Acceptance Criteria" || got[1].Body != "" {
		t.Fatalf("second section = %+v, want empty Acceptance Criteria body", got[1])
	}
}

func TestParseIssue_populates_description_and_acceptance_criteria(t *testing.T) {
	content := `---
key: POP-1
type: story
project: project:POP
schema_version: "1"
---
## Description
This is the description.

## Acceptance Criteria
- one thing
- another thing
`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}

	if issue.Sections == nil {
		t.Fatal("Sections should be populated")
	}
	if issue.Sections[SecDescription] != "This is the description." {
		t.Errorf("Sections[Description] = %q, want %q", issue.Sections[SecDescription], "This is the description.")
	}
	if issue.Sections[SecAcceptanceCriteria] != "- one thing\n- another thing" {
		t.Errorf("Sections[Acceptance Criteria] = %q, want %q", issue.Sections[SecAcceptanceCriteria], "- one thing\n- another thing")
	}
}

func TestParseIssue_sections_missing_always_present(t *testing.T) {
	content := `---
key: MISS-1
type: story
project: project:MISS
schema_version: "1"
---
## Notes
Just some notes.
`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}

	if issue.Sections == nil {
		t.Fatal("Sections should be populated")
	}
	// Missing sections should still be present as empty strings.
	if _, ok := issue.Sections[SecDescription]; !ok {
		t.Errorf("Sections missing key for %q", SecDescription)
	}
	if issue.Sections[SecDescription] != "" {
		t.Errorf("Sections[Description] = %q, want empty", issue.Sections[SecDescription])
	}
	if _, ok := issue.Sections[SecAcceptanceCriteria]; !ok {
		t.Errorf("Sections missing key for %q", SecAcceptanceCriteria)
	}
	if issue.Sections[SecAcceptanceCriteria] != "" {
		t.Errorf("Sections[Acceptance Criteria] = %q, want empty", issue.Sections[SecAcceptanceCriteria])
	}
	// Notes should be present with its body.
	if issue.Sections[SecNotes] != "Just some notes." {
		t.Errorf("Sections[Notes] = %q, want %q", issue.Sections[SecNotes], "Just some notes.")
	}
}

func TestParseIssue_no_body_sections_nil(t *testing.T) {
	content := `---
key: NOBODY-1
type: story
project: project:NOBODY
schema_version: "1"
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}

	// No body means Sections stays nil.
	if issue.Sections != nil {
		t.Errorf("Sections should be nil when there is no body, got %v", issue.Sections)
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
