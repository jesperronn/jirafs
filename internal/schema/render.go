package schema

import (
	"fmt"
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

	// 3b. Read-only Jira context (machine-owned; ignored by plan/push).
	c := i.Context
	for _, kv := range [][2]string{
		{"status", c.Status}, {"priority", c.Priority}, {"reporter", c.Reporter},
		{"created", c.Created}, {"updated", c.Updated},
	} {
		if kv[1] != "" {
			sb.WriteString(kv[0] + ": " + quoteScalar(kv[1]) + "\n")
		}
	}
	if len(c.Components) > 0 {
		sb.WriteString("components:\n")
		for _, comp := range c.Components {
			sb.WriteString("- " + quoteScalar(comp) + "\n")
		}
	}
	if c.Parent != "" {
		sb.WriteString("parent: " + quoteScalar(c.Parent) + "\n")
	}
	if c.EpicLink != "" {
		sb.WriteString("epic_link: " + quoteScalar(c.EpicLink) + "\n")
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
			sb.WriteString("  relation: " + quoteScalar(li.Relation) + "\n")
			if li.LinkType != "" {
				sb.WriteString("  link_type: " + quoteScalar(li.LinkType) + "\n")
			}
			if li.Direction != "" {
				sb.WriteString("  direction: " + quoteScalar(li.Direction) + "\n")
			}
			if li.Summary != "" {
				sb.WriteString("  summary: " + quoteScalar(li.Summary) + "\n")
			}
			if li.Status != "" {
				sb.WriteString("  status: " + quoteScalar(li.Status) + "\n")
			}
		}
	}
	if len(i.FixVersions) > 0 {
		sb.WriteString("fix_versions:\n")
		for _, v := range i.FixVersions {
			sb.WriteString("- " + quoteScalar(v) + "\n")
		}
	}
	if len(i.Subtasks) > 0 {
		sb.WriteString("subtasks:\n")
		for _, st := range i.Subtasks {
			sb.WriteString("- key: " + quoteScalar(st.Key) + "\n")
			if st.Summary != "" {
				sb.WriteString("  summary: " + quoteScalar(st.Summary) + "\n")
			}
			if st.Status != "" {
				sb.WriteString("  status: " + quoteScalar(st.Status) + "\n")
			}
		}
	}

	sb.WriteString(renderCustomFields(i.CustomFields))
	sb.WriteString("---\n")

	// Render fixed sections in stable canonical order.
	sections := i.Sections
	if i.Description != "" && sections[SecDescription] == "" {
		merged := make(map[FixedSectionName]string, len(sections)+1)
		for k, v := range sections {
			merged[k] = v
		}
		merged[SecDescription] = i.Description
		sections = merged
	}
	if len(sections) > 0 {
		sb.WriteString(RenderSections(sections))
	}

	// Machine-owned comments section: rendered from fetched comments, or
	// preserved verbatim from a parsed file.
	if len(i.Comments) > 0 {
		sb.WriteString(renderComments(i.Comments, i.CommentsTotal))
	} else if body := sections[SecComments]; body != "" {
		sb.WriteString("## " + string(SecComments) + "\n" + body + "\n\n")
	}

	return sb.String()
}

// renderComments renders the read-only "## Comments" section, newest first as
// given. Body lines that would read as a section heading are escaped.
func renderComments(comments []Comment, total int) string {
	var sb strings.Builder
	sb.WriteString("## " + string(SecComments) + "\n")
	if total > len(comments) {
		fmt.Fprintf(&sb, "_(showing %d of %d comments)_\n", len(comments), total)
	}
	for _, c := range comments {
		sb.WriteString("\n### " + c.Author + " \u2014 " + c.Created + "\n")
		body := strings.TrimSpace(c.Body)
		if body == "" {
			continue
		}
		lines := strings.Split(body, "\n")
		for idx, l := range lines {
			if strings.HasPrefix(l, "## ") {
				lines[idx] = "\\" + l
			}
		}
		sb.WriteString(strings.Join(lines, "\n") + "\n")
	}
	sb.WriteString("\n")
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
