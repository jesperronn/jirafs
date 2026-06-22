package schema

import (
	"testing"
	"time"
)

// B005d: zero-value, partial-metadata, and invalid-state edge cases.

func TestParseIssue_map_format_project(t *testing.T) {
	// When project is a map {type: project, value: ABC} instead of
	// the flat "project:ABC" string, ParseIssue should still resolve it.
	content := `---
key: MAP-1
type: story
project:
  type: project
  value: MAP
schema_version: "1"
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	if issue.Identity.Key != "MAP-1" {
		t.Errorf("Identity.Key = %q, want %q", issue.Identity.Key, "MAP-1")
	}
	if !issue.Identity.Project.Equals(TypedRef{Type: RefProject, Value: "MAP"}) {
		t.Errorf("Identity.Project = %+v, want {Type: project, Value: MAP}",
			issue.Identity.Project)
	}
}

func TestParseIssue_map_format_project_partial(t *testing.T) {
	// Map format with only type present (no value) should leave project zero.
	content := `---
key: PARTIAL-1
type: story
project:
  type: project
schema_version: "1"
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	if !issue.Identity.Project.IsZero() {
		t.Errorf("Identity.Project should be zero when value is missing, got %+v",
			issue.Identity.Project)
	}
}

func TestParseIssue_nested_remote_metadata(t *testing.T) {
	// When remote metadata is nested under a remote_metadata key instead
	// of flat at the top level, ParseIssue should extract it.
	content := `---
key: NESTED-1
type: story
project: project:NESTED
schema_version: "1"
remote_metadata:
  remote_version: "7"
  content_hash: "nestedhash"
  sync_time: "2026-05-10T14:00:00Z"
  resolved_status: "Done"
  pinned: true
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	if issue.RemoteMetadata.RemoteVersion != "7" {
		t.Errorf("RemoteMetadata.RemoteVersion = %q, want %q",
			issue.RemoteMetadata.RemoteVersion, "7")
	}
	if issue.RemoteMetadata.ContentHash != "nestedhash" {
		t.Errorf("RemoteMetadata.ContentHash = %q, want %q",
			issue.RemoteMetadata.ContentHash, "nestedhash")
	}
	wantTime, _ := time.Parse(time.RFC3339, "2026-05-10T14:00:00Z")
	if !issue.RemoteMetadata.SyncTime.Equal(wantTime) {
		t.Errorf("RemoteMetadata.SyncTime = %v, want %v",
			issue.RemoteMetadata.SyncTime, wantTime)
	}
	if issue.RemoteMetadata.ResolvedStatus != "Done" {
		t.Errorf("RemoteMetadata.ResolvedStatus = %q, want %q",
			issue.RemoteMetadata.ResolvedStatus, "Done")
	}
	if !issue.RemoteMetadata.Pinned {
		t.Error("RemoteMetadata.Pinned should be true")
	}
}

func TestParseIssue_nested_remote_metadata_partial(t *testing.T) {
	// Nested remote_metadata with only some fields present.
	content := `---
key: NESTPART-1
type: story
project: project:NESTPART
schema_version: "1"
remote_metadata:
  remote_version: "3"
  content_hash: "partialhash"
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	if issue.RemoteMetadata.RemoteVersion != "3" {
		t.Errorf("RemoteMetadata.RemoteVersion = %q, want %q",
			issue.RemoteMetadata.RemoteVersion, "3")
	}
	// SyncTime should be zero when not provided in nested format.
	if !issue.RemoteMetadata.SyncTime.IsZero() {
		t.Errorf("RemoteMetadata.SyncTime should be zero, got %v",
			issue.RemoteMetadata.SyncTime)
	}
}

func TestParseIssue_no_sync_time(t *testing.T) {
	// An issue with remote metadata but no sync_time should parse
	// successfully and leave SyncTime as zero.
	content := `---
key: NOSYNC-1
type: story
project: project:NOSYNC
schema_version: "1"
remote_version: "5"
content_hash: "nosynckey"
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	if issue.RemoteMetadata.RemoteVersion != "5" {
		t.Errorf("RemoteMetadata.RemoteVersion = %q, want %q",
			issue.RemoteMetadata.RemoteVersion, "5")
	}
	if issue.RemoteMetadata.ContentHash != "nosynckey" {
		t.Errorf("RemoteMetadata.ContentHash = %q, want %q",
			issue.RemoteMetadata.ContentHash, "nosynckey")
	}
	// SyncTime should be zero when not provided.
	if !issue.RemoteMetadata.SyncTime.IsZero() {
		t.Errorf("RemoteMetadata.SyncTime should be zero, got %v",
			issue.RemoteMetadata.SyncTime)
	}
	// State should be synced because remote metadata is present.
	if issue.RemoteMetadata.State() != StateSynced {
		t.Errorf("RemoteMetadata.State() = %q, want %q",
			issue.RemoteMetadata.State(), StateSynced)
	}
}

func TestParseIssue_no_sync_time_no_metadata(t *testing.T) {
	// An issue with no remote metadata at all should have
	// RemoteMetadata.IsZero() == true and State() == StateUnsynced.
	content := `---
key: NOMETA-1
type: story
project: project:NOMETA
schema_version: "1"
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	if !issue.RemoteMetadata.IsZero() {
		t.Error("RemoteMetadata should be zero when no remote fields are present")
	}
	if issue.RemoteMetadata.State() != StateUnsynced {
		t.Errorf("RemoteMetadata.State() = %q, want %q",
			issue.RemoteMetadata.State(), StateUnsynced)
	}
}

func TestParseIssue_empty_body_sections_nil(t *testing.T) {
	// A frontmatter-only issue (no body at all) should have nil Sections.
	content := `---
key: NODY-1
type: story
project: project:NODY
schema_version: "1"
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	if issue.Sections != nil {
		t.Errorf("Sections should be nil for frontmatter-only issue, got %v",
			issue.Sections)
	}
}

func TestParseIssue_frontmatter_only_with_state(t *testing.T) {
	// A frontmatter-only issue with state: draft should be parseable.
	content := `---
key: FRONTDRF-1
type: story
project: project:FRONTDRF
schema_version: "1"
state: draft
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	if issue.RemoteMetadata.State() != StateDraft {
		t.Errorf("RemoteMetadata.State() = %q, want %q",
			issue.RemoteMetadata.State(), StateDraft)
	}
	if issue.Sections != nil {
		t.Errorf("Sections should be nil, got %v", issue.Sections)
	}
}

func TestParseIssue_invalid_state_value(t *testing.T) {
	// Unknown state values (not "draft" or "archived") with no remote
	// metadata should result in StateUnsynced.
	content := `---
key: BADSTATE-1
type: story
project: project:BADSTATE
schema_version: "1"
state: unknown_value
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	// Unknown state values fall through to IsZero check which returns StateUnsynced.
	if issue.RemoteMetadata.State() != StateUnsynced {
		t.Errorf("RemoteMetadata.State() = %q, want %q",
			issue.RemoteMetadata.State(), StateUnsynced)
	}
}

func TestParseIssue_all_editable_fields_with_map_project(t *testing.T) {
	// A fully populated issue with map-format project to exercise the full
	// parse path including all editable field extraction.
	content := `---
key: FULL-1
type: bug
project:
  type: project
  value: FULL
schema_version: "1"
summary: "Full test issue"
labels:
  - critical
  - regression
assignee: testuser
linked_issues:
  - key: DEP-1
    type: blocks
  - key: DEP-2
    type: relates to
---
## Description
Full description.

## Acceptance Criteria
- AC one
- AC two
`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	if issue.Identity.Key != "FULL-1" {
		t.Errorf("Key = %q, want %q", issue.Identity.Key, "FULL-1")
	}
	if issue.Summary != "Full test issue" {
		t.Errorf("Summary = %q, want %q", issue.Summary, "Full test issue")
	}
	if len(issue.Labels) != 2 || issue.Labels[0] != "critical" {
		t.Errorf("Labels = %v, want [critical regression]", issue.Labels)
	}
	if issue.Assignee == nil || *issue.Assignee != "testuser" {
		t.Errorf("Assignee = %v, want %q", issue.Assignee, "testuser")
	}
	if len(issue.LinkedIssues) != 2 {
		t.Errorf("LinkedIssues len = %d, want 2", len(issue.LinkedIssues))
	}
	if issue.LinkedIssues[0].Key != "DEP-1" || issue.LinkedIssues[0].Type != "blocks" {
		t.Errorf("LinkedIssues[0] = %+v", issue.LinkedIssues[0])
	}
	if issue.Sections[SecDescription] != "Full description." {
		t.Errorf("Sections[Description] = %q", issue.Sections[SecDescription])
	}
}

func TestParseIssue_frontmatter_only_all_editable_fields(t *testing.T) {
	// Frontmatter-only issue with all editable fields (no body).
	content := `---
key: EDITABLE-1
type: task
project: project:EDITABLE
schema_version: "1"
summary: "Editable-only test"
labels:
  - triaged
assignee: assignee-user
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	if issue.Summary != "Editable-only test" {
		t.Errorf("Summary = %q, want %q", issue.Summary, "Editable-only test")
	}
	if len(issue.Labels) != 1 || issue.Labels[0] != "triaged" {
		t.Errorf("Labels = %v, want [triaged]", issue.Labels)
	}
	if issue.Assignee == nil || *issue.Assignee != "assignee-user" {
		t.Errorf("Assignee = %v, want %q", issue.Assignee, "assignee-user")
	}
}

func TestParseIssue_empty_string_fields(t *testing.T) {
	// Fields that parse as empty strings should be treated as zero.
	content := `---
key: EMPTY-1
type: story
project: project:EMPTY
schema_version: "1"
summary: ""
labels: []
assignee: ""
---`

	issue, err := ParseIssue(content)
	if err != nil {
		t.Fatalf("ParseIssue returned error: %v", err)
	}
	// Empty summary should be treated as zero (not set).
	if issue.Summary != "" {
		t.Errorf("Summary = %q, want empty", issue.Summary)
	}
	// Empty labels should result in nil/empty slice.
	if len(issue.Labels) != 0 {
		t.Errorf("Labels = %v, want empty", issue.Labels)
	}
	// Empty assignee should be nil (not a pointer to empty string).
	if issue.Assignee != nil {
		t.Errorf("Assignee should be nil for empty string, got %v", issue.Assignee)
	}
}

// splitSectionBlocks edge-case tests.

func TestSplitSectionBlocks_empty_body(t *testing.T) {
	got := splitSectionBlocks("")
	if got != nil {
		t.Errorf("splitSectionBlocks(\"\") = %v, want nil", got)
	}
}

func TestSplitSectionBlocks_whitespace_only_body(t *testing.T) {
	got := splitSectionBlocks("   \n  \n  ")
	if got != nil {
		t.Errorf("splitSectionBlocks whitespace = %v, want nil", got)
	}
}

func TestSplitSectionBlocks_body_without_headings(t *testing.T) {
	body := "Just plain text with no headings.\nAnother line."
	got := splitSectionBlocks(body)
	if got != nil {
		t.Errorf("splitSectionBlocks no headings = %v, want nil", got)
	}
}

func TestSplitSectionBlocks_single_section_no_body(t *testing.T) {
	body := "## Description"
	got := splitSectionBlocks(body)
	if len(got) != 1 {
		t.Fatalf("splitSectionBlocks single section = %d blocks, want 1", len(got))
	}
	if got[0].Heading != "Description" {
		t.Errorf("Heading = %q, want %q", got[0].Heading, "Description")
	}
	if got[0].Body != "" {
		t.Errorf("Body = %q, want empty", got[0].Body)
	}
}

func TestSplitSectionBlocks_trailing_newlines(t *testing.T) {
	body := `## Description
Some text.

## Notes
Notes here.

`
	got := splitSectionBlocks(body)
	if len(got) != 2 {
		t.Fatalf("splitSectionBlocks = %d blocks, want 2", len(got))
	}
	if got[0].Body != "Some text." {
		t.Errorf("first body = %q, want %q", got[0].Body, "Some text.")
	}
	if got[1].Body != "Notes here." {
		t.Errorf("second body = %q, want %q", got[1].Body, "Notes here.")
	}
}

func TestExtractFrontmatter_no_frontmatter_delimiter(t *testing.T) {
	_, _, err := extractFrontmatter("no frontmatter here")
	if err == nil {
		t.Fatal("expected error for no frontmatter")
	}
	if err.Kind != ErrKindNoFrontmatter {
		t.Errorf("Kind = %q, want %q", err.Kind, ErrKindNoFrontmatter)
	}
}

func TestExtractFrontmatter_only_opening_delimiter(t *testing.T) {
	_, _, err := extractFrontmatter("---")
	if err == nil {
		t.Fatal("expected error for only opening delimiter")
	}
	if err.Kind != ErrKindNoClosingDelimiter {
		t.Errorf("Kind = %q, want %q", err.Kind, ErrKindNoClosingDelimiter)
	}
}

func TestExtractFrontmatter_content_before_delimiter_rejected(t *testing.T) {
	// extractFrontmatter requires content to start with "---".
	// Content before the delimiter is rejected.
	content := `some prefix
---
key: ABC-1
type: story
---
## Description
Body text.
`
	_, _, err := extractFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for content before delimiter")
	}
	if err.Kind != ErrKindNoFrontmatter {
		t.Errorf("Kind = %q, want %q", err.Kind, ErrKindNoFrontmatter)
	}
}

func TestExtractFrontmatter_empty_frontmatter(t *testing.T) {
	_, _, err := extractFrontmatter("---\n---\nbody")
	if err != nil {
		t.Fatalf("extractFrontmatter returned error: %v", err)
	}
}
