package sync

import (
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestApplyOpPriorityCustomFieldsAndClears(t *testing.T) {
	r := schema.Issue{CustomFields: map[string]interface{}{"customfield_1": "a"}}
	for _, op := range []schema.PlanOperation{
		{Field: schema.EditableFieldPriority, Type: schema.OpSet, Value: "Low"},
		{Field: schema.EditableFieldCustomFields, Type: schema.OpSet, Value: `{"customfield_2":"b"}`},
		{Field: schema.EditableFieldCustomFields, Type: schema.OpSet, Value: `not json`},
		{Field: schema.EditableFieldLabels, Type: schema.OpSet, Value: ""},
		{Field: schema.EditableFieldFixVersions, Type: schema.OpSet, Value: ""},
	} {
		applyOp(&r, op)
	}
	if r.Context.Priority != "Low" || len(r.CustomFields) != 2 || len(r.ClearFields) != 3 {
		t.Errorf("%+v", r)
	}
}
