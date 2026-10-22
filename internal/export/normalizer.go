package export

import (
	"github.com/jirafs/jirafs/internal/schema"
)

// NormalizeLinkedIssues extracts linked issue references from a Jira fields
// map and writes them into issue.LinkedIssues.
//
// Jira returns linked issues under the "issuelinks" key as an array of link
// objects. Each link may have an "outwardIssue" or "issue" field pointing to
// the target, plus a "type" object with the link name.
func NormalizeLinkedIssues(issue *schema.Issue, fields map[string]interface{}) {
	if fields == nil {
		return
	}

	links, ok := fields["issuelinks"].([]interface{})
	if !ok || len(links) == 0 {
		issue.LinkedIssues = []schema.LinkedIssue{}
		return
	}

	// Deduplicate by key plus relation: the same issue may legitimately be
	// linked twice with different relations.
	seen := make(map[string]bool)
	result := make([]schema.LinkedIssue, 0)

	for _, raw := range links {
		link, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		// outwardIssue carries the type's outward text; issue (the inward
		// side) carries the inward text.
		var target map[string]interface{}
		var relKey string
		if outward, ok := link["outwardIssue"].(map[string]interface{}); ok {
			target, relKey = outward, "outward"
		} else if inward, ok := link["issue"].(map[string]interface{}); ok {
			target, relKey = inward, "inward"
		}
		targetKey, _ := target["key"].(string)
		if targetKey == "" {
			continue
		}

		// The directional text comes from the link type; fall back to the
		// type name when Jira omits it.
		var relation, linkType string
		if lt, ok := link["type"].(map[string]interface{}); ok {
			linkType, _ = lt["name"].(string)
			relation, _ = lt[relKey].(string)
			if relation == "" {
				relation, _ = lt["name"].(string)
			}
		}

		id := targetKey + "\x00" + relation
		if seen[id] {
			continue
		}
		seen[id] = true

		li := schema.LinkedIssue{Key: schema.IssueKey(targetKey), Relation: relation, LinkType: linkType, Direction: relKey}
		if tf, ok := target["fields"].(map[string]interface{}); ok {
			li.Summary, _ = tf["summary"].(string)
			li.Status = nestedName(tf["status"])
		}
		result = append(result, li)
	}

	issue.LinkedIssues = result
}

// ExportIssue renders a populated *schema.Issue through the canonical codec
// (schema.RenderIssue) and returns the resulting YAML frontmatter + sections
// as a string. The output is suitable for writing directly to a local issue
// file and will round-trip through schema.ParseIssue.
func ExportIssue(issue *schema.Issue) string {
	return schema.RenderIssue(*issue)
}

// NormalizeIssue extracts summary, description, labels, and assignee from a
// Jira fields map and writes them into the issue model.
func NormalizeIssue(issue *schema.Issue, fields map[string]interface{}) {
	if fields == nil {
		return
	}

	if s, ok := fields["summary"].(string); ok {
		issue.Summary = s
	}

	if d, ok := fields["description"].(string); ok {
		issue.Description = d
	}

	if labels, ok := fields["labels"].([]interface{}); ok {
		ls := make([]string, 0, len(labels))
		for _, l := range labels {
			if s, ok := l.(string); ok {
				ls = append(ls, s)
			}
		}
		issue.Labels = ls
	}

	issue.FixVersions = nil
	if fvs, ok := fields["fixVersions"].([]interface{}); ok {
		for _, raw := range fvs {
			if n := nestedName(raw); n != "" {
				issue.FixVersions = append(issue.FixVersions, n)
			}
		}
	}

	issue.Subtasks = nil
	if sts, ok := fields["subtasks"].([]interface{}); ok {
		for _, raw := range sts {
			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			key, _ := m["key"].(string)
			if key == "" {
				continue
			}
			st := schema.Subtask{Key: key}
			if sf, ok := m["fields"].(map[string]interface{}); ok {
				st.Summary, _ = sf["summary"].(string)
				st.Status = nestedName(sf["status"])
			}
			issue.Subtasks = append(issue.Subtasks, st)
		}
	}

	if assignee, ok := fields["assignee"].(map[string]interface{}); ok {
		if name, ok := assignee["name"].(string); ok {
			issue.Assignee = &name
		}
	}
}

// NormalizeContext extracts the read-only Jira context fields (status,
// priority, reporter, created, updated, components, parent and, when
// epicLinkField names a custom field, the epic link) into issue.Context.
func NormalizeContext(issue *schema.Issue, fields map[string]interface{}, epicLinkField string) {
	if fields == nil {
		return
	}
	c := &issue.Context
	c.Status = nestedName(fields["status"])
	c.Priority = nestedName(fields["priority"])
	c.Reporter = userName(fields["reporter"])
	if s, ok := fields["created"].(string); ok {
		c.Created = s
	}
	if s, ok := fields["updated"].(string); ok {
		c.Updated = s
	}
	c.Components = nil
	if comps, ok := fields["components"].([]interface{}); ok {
		for _, raw := range comps {
			if n := nestedName(raw); n != "" {
				c.Components = append(c.Components, n)
			}
		}
	}
	if p, ok := fields["project"].(map[string]interface{}); ok {
		if k, _ := p["key"].(string); k != "" {
			issue.Identity.Project = schema.TypedRef{Type: schema.RefProject, Value: k}
		}
	}
	c.Parent = ""
	if p, ok := fields["parent"].(map[string]interface{}); ok {
		if k, ok := p["key"].(string); ok {
			c.Parent = k
		}
	}
	c.EpicLink = ""
	if epicLinkField != "" {
		if s, ok := fields[epicLinkField].(string); ok {
			c.EpicLink = s
		}
	}
}

func nestedName(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	s, _ := m["name"].(string)
	return s
}

func userName(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	for _, k := range []string{"displayName", "name", "emailAddress"} {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// NormalizeCustomFieldValues extracts the listed custom fields from a Jira
// fields map into issue.CustomFields in the shape drafts use for create:
// option objects become {value: ...}, strings stay strings, nulls and empty
// lists are omitted. names (id to display name) is stored read-only in
// issue.Context.CustomFieldNames for the listed ids.
func NormalizeCustomFieldValues(issue *schema.Issue, fields map[string]interface{}, ids []string, names map[string]string) {
	if fields == nil || len(ids) == 0 {
		return
	}
	issue.CustomFields = nil
	issue.Context.CustomFieldNames = nil
	for _, id := range ids {
		if v, ok := normalizeCustomValue(fields[id]); ok {
			if issue.CustomFields == nil {
				issue.CustomFields = map[string]interface{}{}
			}
			issue.CustomFields[id] = v
		}
		if n := names[id]; n != "" {
			if issue.Context.CustomFieldNames == nil {
				issue.Context.CustomFieldNames = map[string]string{}
			}
			issue.Context.CustomFieldNames[id] = n
		}
	}
}

func normalizeCustomValue(v interface{}) (interface{}, bool) {
	switch t := v.(type) {
	case nil:
		return nil, false
	case float64:
		if t == float64(int64(t)) {
			return int64(t), true
		}
		return t, true
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, e := range t {
			if n, ok := normalizeCustomValue(e); ok {
				out = append(out, n)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	case map[string]interface{}:
		if val, ok := t["value"].(string); ok {
			return map[string]interface{}{"value": val}, true
		}
		if name, ok := t["name"].(string); ok {
			return map[string]interface{}{"name": name}, true
		}
		return t, true
	default:
		return v, true
	}
}
