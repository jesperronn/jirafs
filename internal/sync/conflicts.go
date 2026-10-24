package sync

import (
	"encoding/json"
	"sort"

	"github.com/jirafs/jirafs/internal/plan"
	"github.com/jirafs/jirafs/internal/schema"
)

// scalarFields are the editable fields compared as one value each.
var scalarFields = []schema.EditableField{
	schema.EditableFieldSummary,
	schema.EditableFieldDescription,
	schema.EditableFieldStatus,
	schema.EditableFieldLabels,
	schema.EditableFieldFixVersions,
	schema.EditableFieldAssignee,
	schema.EditableFieldSprint,
	schema.EditableFieldPriority,
}

// DetectConflicts compares every editable field of local and remote against
// base and returns a ConflictBothEdited, naming the real field and carrying
// both values, for each field that differs from base on both sides and
// differs between the sides. A field changed on one side only is no conflict.
//
// A nil base means no snapshot is known: every field where local differs from
// remote is reported, since neither side can be proven unchanged. Version and
// hash metadata are not consulted, so their absence never skips the check.
// Status and priority that are empty locally mean "not set locally" and are
// never compared.
func DetectConflicts(base *schema.Issue, local, remote schema.Issue) []schema.Conflict {
	var out []schema.Conflict
	for _, f := range scalarFields {
		lv, rv := plan.FieldValue(local, f), plan.FieldValue(remote, f)
		if (f == schema.EditableFieldStatus || f == schema.EditableFieldPriority) && lv == "" {
			continue
		}
		if f == schema.EditableFieldStatus && rv == "" {
			continue
		}
		if plan.SameFieldValue(f, lv, rv) {
			continue
		}
		if base != nil {
			bv := plan.FieldValue(*base, f)
			if plan.SameFieldValue(f, lv, bv) || plan.SameFieldValue(f, rv, bv) {
				continue
			}
		}
		out = append(out, schema.Conflict{Field: f, Type: schema.ConflictBothEdited, LocalValue: lv, RemoteValue: rv})
	}
	if c, ok := customFieldConflict(base, local, remote); ok {
		out = append(out, c)
	}
	return out
}

// customFieldConflict reports the custom field keys changed on both sides to
// different values, as the JSON objects of those keys.
func customFieldConflict(base *schema.Issue, local, remote schema.Issue) (schema.Conflict, bool) {
	changed := plan.ChangedCustomFields(local.CustomFields, remote.CustomFields)
	lm, rm := map[string]interface{}{}, map[string]interface{}{}
	keys := make([]string, 0, len(changed))
	for k := range changed {
		if base != nil {
			sameLocal := len(plan.ChangedCustomFields(map[string]interface{}{k: local.CustomFields[k]}, base.CustomFields)) == 0
			sameRemote := len(plan.ChangedCustomFields(map[string]interface{}{k: remote.CustomFields[k]}, base.CustomFields)) == 0
			if sameLocal || sameRemote {
				continue
			}
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return schema.Conflict{}, false
	}
	sort.Strings(keys)
	for _, k := range keys {
		lm[k] = local.CustomFields[k]
		rm[k] = remote.CustomFields[k]
	}
	lb, _ := json.Marshal(lm)
	rb, _ := json.Marshal(rm)
	return schema.Conflict{
		Field:       schema.EditableFieldCustomFields,
		Type:        schema.ConflictBothEdited,
		LocalValue:  string(lb),
		RemoteValue: string(rb),
	}, true
}
