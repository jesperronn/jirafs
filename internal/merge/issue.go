package merge

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/jirafs/jirafs/internal/plan"
	"github.com/jirafs/jirafs/internal/schema"
)

// FieldLinkedIssues names the linked_issues list in a Conflict.
const FieldLinkedIssues schema.EditableField = "linked_issues"

// Conflict is one field changed on both sides. Remote and Base are the
// comparable string forms (plan.FieldValue) of the remote and base values; the
// local value stays in the merged issue.
type Conflict struct {
	Field  schema.EditableField
	Remote string
	Base   string
}

// Result is the outcome of merging one issue.
type Result struct {
	// Merged is the issue to write: remote content with local-only changes
	// applied, and the local value for every conflicted field. Local body
	// sections other than Description are preserved.
	Merged schema.Issue
	// NewBase is the snapshot to store as the next base: the remote issue with
	// conflicted fields reset to their old base value so the next pull
	// reproduces them. Meaningful only when a base existed.
	NewBase schema.Issue
	// Conflicts lists the fields changed on both sides, in a stable order.
	Conflicts []Conflict
}

// Issue three-way merges local and remote against base. A nil base means no
// snapshot exists: every field where local differs from remote is conflicted.
// Status and priority that are empty locally are "not set locally" and take
// the remote value; an empty remote status never overwrites local.
func Issue(base *schema.Issue, local, remote schema.Issue) Result {
	merged := remote
	newBase := remote
	var conflicts []Conflict

	fields := []schema.EditableField{
		schema.EditableFieldSummary,
		schema.EditableFieldDescription,
		schema.EditableFieldLabels,
		schema.EditableFieldFixVersions,
		schema.EditableFieldAssignee,
		schema.EditableFieldSprint,
		schema.EditableFieldStatus,
		schema.EditableFieldPriority,
		FieldLinkedIssues,
	}
	for _, f := range fields {
		lv, rv := value(local, f), value(remote, f)
		bv := ""
		if base != nil {
			bv = value(*base, f)
		}
		switch {
		case (f == schema.EditableFieldStatus || f == schema.EditableFieldPriority) && lv == "":
			continue
		case f == schema.EditableFieldStatus && rv == "":
			copyField(&merged, local, f)
			continue
		case same(f, lv, rv):
			continue
		case base != nil && same(f, lv, bv):
			continue // only remote changed: merged already holds remote
		case base != nil && same(f, rv, bv):
			copyField(&merged, local, f) // only local changed
			continue
		}
		if f == schema.EditableFieldDescription && base != nil {
			if text, ok := Lines(base.Description, local.Description, remote.Description); ok {
				merged.Description = text
				continue
			}
		}
		copyField(&merged, local, f)
		if base != nil {
			copyField(&newBase, *base, f)
		}
		conflicts = append(conflicts, Conflict{Field: f, Remote: rv, Base: bv})
	}

	if c, ok := mergeCustomFields(base, local, remote, &merged, &newBase); ok {
		conflicts = append(conflicts, c)
	}

	merged.Sections = localSections(local, merged.Description)
	return Result{Merged: merged, NewBase: newBase, Conflicts: conflicts}
}

// localSections keeps the local body sections (Notes, Comments To Add, ...)
// with the merged description. Empty sections and the machine-owned Comments
// section are dropped; they are rendered from the remote.
func localSections(local schema.Issue, description string) map[schema.FixedSectionName]string {
	out := map[schema.FixedSectionName]string{}
	for name, body := range local.Sections {
		if body != "" && name != schema.SecComments {
			out[name] = body
		}
	}
	if description != "" {
		out[schema.SecDescription] = description
	} else {
		delete(out, schema.SecDescription)
	}
	return out
}

func value(i schema.Issue, f schema.EditableField) string {
	if f == FieldLinkedIssues {
		parts := make([]string, 0, len(i.LinkedIssues))
		for _, li := range i.LinkedIssues {
			parts = append(parts, string(li.Key)+"|"+li.Relation+"|"+li.LinkType+"|"+li.Direction)
		}
		sort.Strings(parts)
		return strings.Join(parts, "\n")
	}
	return plan.FieldValue(i, f)
}

func same(f schema.EditableField, a, b string) bool {
	return plan.SameFieldValue(f, a, b)
}

// copyField sets field f of dst to its value in src.
func copyField(dst *schema.Issue, src schema.Issue, f schema.EditableField) {
	switch f {
	case schema.EditableFieldSummary:
		dst.Summary = src.Summary
	case schema.EditableFieldDescription:
		dst.Description = src.Description
	case schema.EditableFieldLabels:
		dst.Labels = append([]string(nil), src.Labels...)
	case schema.EditableFieldFixVersions:
		dst.FixVersions = append([]string(nil), src.FixVersions...)
	case schema.EditableFieldAssignee:
		dst.Assignee = src.Assignee
	case schema.EditableFieldSprint:
		dst.Sprint = src.Sprint
	case schema.EditableFieldStatus:
		dst.Status = src.Status
		dst.Context.Status = src.Context.Status
	case schema.EditableFieldPriority:
		dst.Context.Priority = src.Context.Priority
	case FieldLinkedIssues:
		dst.LinkedIssues = append([]schema.LinkedIssue(nil), src.LinkedIssues...)
	}
}

// customValue returns the JSON form of key in m, or "\x00" when absent.
func customValue(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return "\x00"
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// mergeCustomFields merges custom_fields key by key into merged and newBase and
// reports the keys changed on both sides as one custom_fields conflict whose
// Remote and Base are JSON objects of those keys.
func mergeCustomFields(base *schema.Issue, local, remote schema.Issue, merged, newBase *schema.Issue) (Conflict, bool) {
	keys := map[string]bool{}
	for _, m := range []map[string]interface{}{local.CustomFields, remote.CustomFields} {
		for k := range m {
			keys[k] = true
		}
	}
	var bm map[string]interface{}
	if base != nil {
		bm = base.CustomFields
		for k := range bm {
			keys[k] = true
		}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	mc := cloneMap(remote.CustomFields)
	nb := cloneMap(remote.CustomFields)
	rObj, bObj := map[string]interface{}{}, map[string]interface{}{}
	conflicted := false
	for _, k := range sorted {
		lv, rv := customValue(local.CustomFields, k), customValue(remote.CustomFields, k)
		if lv == rv {
			continue
		}
		if base != nil {
			bv := customValue(bm, k)
			if lv == bv {
				continue
			}
			if rv == bv {
				setOrDelete(mc, k, local.CustomFields)
				continue
			}
			setOrDelete(nb, k, bm)
			if v, ok := bm[k]; ok {
				bObj[k] = v
			}
		}
		setOrDelete(mc, k, local.CustomFields)
		if v, ok := remote.CustomFields[k]; ok {
			rObj[k] = v
		}
		conflicted = true
	}
	merged.CustomFields = nilIfEmpty(mc)
	newBase.CustomFields = nilIfEmpty(nb)
	if !conflicted {
		return Conflict{}, false
	}
	rb, _ := json.Marshal(rObj)
	bb, _ := json.Marshal(bObj)
	return Conflict{Field: schema.EditableFieldCustomFields, Remote: string(rb), Base: string(bb)}, true
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func setOrDelete(dst map[string]interface{}, key string, src map[string]interface{}) {
	if v, ok := src[key]; ok {
		dst[key] = v
	} else {
		delete(dst, key)
	}
}

func nilIfEmpty(m map[string]interface{}) map[string]interface{} {
	if len(m) == 0 {
		return nil
	}
	return m
}
