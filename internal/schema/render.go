package schema

import (
	"strings"
	"time"
)

// RenderIssue serializes an Issue back into YAML frontmatter with a stable,
// deterministic field order.  The output is suitable for writing back to an
// issue file and will round-trip through ParseIssue.
//
// Field order (always identical regardless of how the Issue was populated):
//
//	Identity: key, type, project
//	Machine-owned: schema_version
//	State/metadata: state, remote_version, content_hash, sync_time
//	Editable fields: summary, labels, assignee, linked_issues
//
// Fields that are zero-valued are omitted (omitempty semantics).
func RenderIssue(i Issue) string {
	var sb strings.Builder

	sb.WriteString("---\n")

	// 1. Identity fields.
	if i.Identity.Key != "" {
		sb.WriteString("key: " + quoteScalar(string(i.Identity.Key)) + "\n")
	}
	if i.Identity.Type != "" {
		sb.WriteString("type: " + quoteScalar(string(i.Identity.Type)) + "\n")
	}
	if !i.Identity.Project.IsZero() {
		sb.WriteString("project: " + quoteScalar(i.Identity.Project.String()) + "\n")
	}

	// 2. Machine-owned fields.
	if i.MachineOwned.SchemaVersion != "" {
		sb.WriteString("schema_version: " + quoteScalar(i.MachineOwned.SchemaVersion) + "\n")
	}

	// 3. State / remote metadata fields (only when relevant).
	if i.RemoteMetadata.StateFile != "" {
		sb.WriteString("state: " + quoteScalar(i.RemoteMetadata.StateFile) + "\n")
	}
	if i.RemoteMetadata.RemoteVersion != "" {
		sb.WriteString("remote_version: " + quoteScalar(i.RemoteMetadata.RemoteVersion) + "\n")
	}
	if i.RemoteMetadata.ContentHash != "" {
		sb.WriteString("content_hash: " + quoteScalar(i.RemoteMetadata.ContentHash) + "\n")
	}
	if !i.RemoteMetadata.SyncTime.IsZero() {
		sb.WriteString("sync_time: " + quoteScalar(i.RemoteMetadata.SyncTime.Format(time.RFC3339)) + "\n")
	}

	// 4. Editable fields.
	if i.Summary != "" {
		sb.WriteString("summary: " + quoteScalar(i.Summary) + "\n")
	}
	if len(i.Labels) > 0 {
		sb.WriteString("labels:\n")
		for _, l := range i.Labels {
			sb.WriteString("- " + quoteScalar(l) + "\n")
		}
	}
	if i.Assignee != nil && *i.Assignee != "" {
		sb.WriteString("assignee: " + quoteScalar(*i.Assignee) + "\n")
	}
	if len(i.LinkedIssues) > 0 {
		sb.WriteString("linked_issues:\n")
		for _, li := range i.LinkedIssues {
			sb.WriteString("- key: " + quoteScalar(string(li.Key)) + "\n")
			sb.WriteString("  type: " + quoteScalar(li.Type) + "\n")
		}
	}

	sb.WriteString("---\n")

	// Render fixed sections in stable canonical order.
	if len(i.Sections) > 0 {
		sb.WriteString(RenderSections(i.Sections))
	}

	return sb.String()
}

// quoteScalar returns the YAML-quoted form of s so that it is always parsed
// back as a plain scalar (not interpreted as a number, boolean, etc.).
func quoteScalar(s string) string {
	// Use single quotes for simplicity and safety.
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// RenderSections renders the fixed sections of an issue body in stable,
// canonical order (the order returned by AllFixedSections).  For every known
// section the heading is always emitted; the body is emitted only when the
// map contains a non-empty entry for that key.
//
// The output is a plain-text block suitable for appending after the
// frontmatter delimiter.
func RenderSections(sections map[FixedSectionName]string) string {
	var sb strings.Builder

	for _, name := range AllFixedSections() {
		sb.WriteString("## " + string(name) + "\n")
		if body, ok := sections[name]; ok && body != "" {
			sb.WriteString(body + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
