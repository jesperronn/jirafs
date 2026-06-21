package schema

import (
	"strings"
	"testing"
	"time"
)

func TestRenderIssue_synced_frontmatter_field_order(t *testing.T) {
	syncTime, _ := time.Parse(time.RFC3339, "2026-06-21T12:00:00Z")

	issue := Issue{
		Identity: IssueIdentity{
			Key:     "ABC-123",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "ABC"},
		},
		MachineOwned: MachineOwned{
			SchemaVersion: "1",
		},
		RemoteMetadata: RemoteMetadata{
			StateFile:     "synced",
			RemoteVersion: "42",
			ContentHash:   "abc123def",
			SyncTime:      syncTime,
		},
	}

	out := RenderIssue(issue)

	// Verify stable field order: key → type → project → schema_version →
	// state → remote_version → content_hash → sync_time.
	wantOrder := []string{
		"key", "type", "project",
		"schema_version",
		"state", "remote_version", "content_hash", "sync_time",
	}

	// Only count top-level fields (start at column 0).
	var fieldOrder []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) > 0 && line[0] != '-' && line[0] != ' ' {
			if idx := strings.Index(line, ":"); idx > 0 {
				fieldOrder = append(fieldOrder, line[:idx])
			}
		}
	}

	if len(fieldOrder) != len(wantOrder) {
		t.Fatalf("top-level field count = %d, want %d; fields: %v", len(fieldOrder), len(wantOrder), fieldOrder)
	}
	for i, want := range wantOrder {
		if fieldOrder[i] != want {
			t.Errorf("field[%d] = %q, want %q (full output:\n%s)", i, fieldOrder[i], want, out)
		}
	}
}

func TestRenderIssue_roundtrips_through_parse(t *testing.T) {
	syncTime, _ := time.Parse(time.RFC3339, "2026-06-21T12:00:00Z")

	original := Issue{
		Identity: IssueIdentity{
			Key:     "RT-1",
			Type:    "bug",
			Project: TypedRef{Type: RefProject, Value: "RT"},
		},
		MachineOwned: MachineOwned{
			SchemaVersion: "1",
		},
		RemoteMetadata: RemoteMetadata{
			StateFile:     "synced",
			RemoteVersion: "10",
			ContentHash:   "hash123",
			SyncTime:      syncTime,
		},
	}

	rendered := RenderIssue(original)
	parsed, err := ParseIssue(rendered)
	if err != nil {
		t.Fatalf("ParseIssue(rendered) returned error: %v\nrendered:\n%s", err, rendered)
	}

	// Verify only the fields that ParseIssue reads from frontmatter.
	if parsed.Identity.Key != original.Identity.Key {
		t.Errorf("round-trip key = %q, want %q", parsed.Identity.Key, original.Identity.Key)
	}
	if parsed.Identity.Type != original.Identity.Type {
		t.Errorf("round-trip type = %q, want %q", parsed.Identity.Type, original.Identity.Type)
	}
	if !parsed.Identity.Project.Equals(original.Identity.Project) {
		t.Errorf("round-trip project = %+v, want %+v", parsed.Identity.Project, original.Identity.Project)
	}
	if parsed.MachineOwned.SchemaVersion != original.MachineOwned.SchemaVersion {
		t.Errorf("round-trip schema_version = %q, want %q", parsed.MachineOwned.SchemaVersion, original.MachineOwned.SchemaVersion)
	}
	if parsed.RemoteMetadata.RemoteVersion != original.RemoteMetadata.RemoteVersion {
		t.Errorf("round-trip remote_version = %q, want %q", parsed.RemoteMetadata.RemoteVersion, original.RemoteMetadata.RemoteVersion)
	}
	if parsed.RemoteMetadata.ContentHash != original.RemoteMetadata.ContentHash {
		t.Errorf("round-trip content_hash = %q, want %q", parsed.RemoteMetadata.ContentHash, original.RemoteMetadata.ContentHash)
	}
	if !parsed.RemoteMetadata.SyncTime.Equal(original.RemoteMetadata.SyncTime) {
		t.Errorf("round-trip sync_time = %v, want %v", parsed.RemoteMetadata.SyncTime, original.RemoteMetadata.SyncTime)
	}
	if parsed.RemoteMetadata.State() != StateSynced {
		t.Errorf("round-trip state = %q, want %q", parsed.RemoteMetadata.State(), StateSynced)
	}
}

func TestRenderIssue_stable_order_same_for_zero_fields(t *testing.T) {
	// Two issues with the same non-zero fields but different zero fields
	// should produce identical output (stable order = no extra fields).
	issue1 := Issue{
		Identity: IssueIdentity{
			Key:     "STABLE-1",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "STABLE"},
		},
		MachineOwned: MachineOwned{
			SchemaVersion: "1",
		},
		Summary: "test",
	}

	issue2 := Issue{
		Identity: IssueIdentity{
			Key:     "STABLE-1",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "STABLE"},
		},
		MachineOwned: MachineOwned{
			SchemaVersion: "1",
		},
		RemoteMetadata: RemoteMetadata{
			// all zero
		},
		Summary: "test",
	}

	out1 := RenderIssue(issue1)
	out2 := RenderIssue(issue2)

	if out1 != out2 {
		t.Errorf("outputs differ for equivalent issues:\n--- issue1 ---\n%s\n--- issue2 ---\n%s", out1, out2)
	}
}

func TestRenderIssue_deterministic_across_renders(t *testing.T) {
	syncTime, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	assignee := "user"

	issue := Issue{
		Identity: IssueIdentity{
			Key:     "DET-1",
			Type:    "task",
			Project: TypedRef{Type: RefProject, Value: "DET"},
		},
		MachineOwned: MachineOwned{SchemaVersion: "1"},
		RemoteMetadata: RemoteMetadata{
			StateFile:     "draft",
			RemoteVersion: "1",
			ContentHash:   "abc",
			SyncTime:      syncTime,
		},
		Summary:    "deterministic",
		Labels:     []string{"z", "a", "m"},
		Assignee:   &assignee,
		LinkedIssues: []LinkedIssue{
			{Key: "X-1", Type: "blocks"},
			{Key: "Y-1", Type: "relates to"},
		},
	}

	// Render the same issue 10 times; all outputs must be identical.
	for i := 0; i < 10; i++ {
		out := RenderIssue(issue)
		if out != RenderIssue(issue) {
			t.Fatalf("render %d produced different output", i)
		}
	}
}

func TestRenderIssue_empty_issue(t *testing.T) {
	out := RenderIssue(Issue{})
	if out != "---\n---\n" {
		t.Errorf("empty issue output = %q, want %q", out, "---\n---\n")
	}
}

func TestRenderIssue_draft_state(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "DRF-1",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "DRF"},
		},
		MachineOwned: MachineOwned{SchemaVersion: "1"},
		RemoteMetadata: RemoteMetadata{
			StateFile: "draft",
		},
		Summary: "draft issue",
	}

	out := RenderIssue(issue)
	if !strings.Contains(out, "state: 'draft'") {
		t.Errorf("output missing state: draft:\n%s", out)
	}
	if !strings.Contains(out, "summary: 'draft issue'") {
		t.Errorf("output missing summary:\n%s", out)
	}
	// No remote_version, content_hash, or sync_time should appear.
	if strings.Contains(out, "remote_version:") {
		t.Error("draft output should not contain remote_version")
	}
	if strings.Contains(out, "content_hash:") {
		t.Error("draft output should not contain content_hash")
	}
	if strings.Contains(out, "sync_time:") {
		t.Error("draft output should not contain sync_time")
	}
}

func TestRenderIssue_quoted_scalars(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "123", // numeric-looking key
			Type:    "true", // boolean-looking type
			Project: TypedRef{Type: RefProject, Value: "null"}, // null-looking project
		},
		MachineOwned: MachineOwned{SchemaVersion: "1"},
	}

	out := RenderIssue(issue)

	// All should be quoted to avoid YAML type coercion.
	if !strings.Contains(out, "'123'") {
		t.Errorf("key should be quoted: %s", out)
	}
	if !strings.Contains(out, "'true'") {
		t.Errorf("type should be quoted: %s", out)
	}
	// The project value "null" is embedded in "project:null" which is quoted.
	if !strings.Contains(out, "'project:null'") {
		t.Errorf("project should be quoted: %s", out)
	}
}

func TestRenderIssue_labels_stable_order(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "LAB-1",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "LAB"},
		},
		MachineOwned: MachineOwned{SchemaVersion: "1"},
		Labels:       []string{"z-label", "a-label", "m-label"},
	}

	out := RenderIssue(issue)
	// Labels should appear in the order they were set (stable).
	zIdx := strings.Index(out, "z-label")
	aIdx := strings.Index(out, "a-label")
	mIdx := strings.Index(out, "m-label")

	if zIdx >= aIdx || aIdx >= mIdx {
		t.Errorf("labels not in insertion order:\n%s", out)
	}
}

func TestRenderIssue_linked_issues_stable_order(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "LI-1",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "LI"},
		},
		MachineOwned: MachineOwned{SchemaVersion: "1"},
		LinkedIssues: []LinkedIssue{
			{Key: "A-1", Type: "blocks"},
			{Key: "B-1", Type: "relates to"},
		},
	}

	out := RenderIssue(issue)
	aIdx := strings.Index(out, "A-1")
	bIdx := strings.Index(out, "B-1")

	if aIdx >= bIdx {
		t.Errorf("linked_issues not in insertion order:\n%s", out)
	}
}
