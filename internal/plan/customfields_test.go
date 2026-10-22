package plan

import (
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestBuildPlanPriorityAndCustomFields(t *testing.T) {
	remote := schema.Issue{Context: schema.IssueContext{Priority: "High"}, CustomFields: map[string]interface{}{"customfield_1": "a", "customfield_2": map[string]interface{}{"value": "X"}}}
	same := schema.Issue{Context: schema.IssueContext{Priority: "High"}, CustomFields: map[string]interface{}{"customfield_1": "a", "customfield_2": map[string]interface{}{"value": "X"}, "customfield_3": nil}}
	if ops, _, _ := BuildPlan(same, remote); len(ops) != 0 {
		t.Fatalf("ops %v", ops)
	}
	local := schema.Issue{Context: schema.IssueContext{Priority: "Low"}, CustomFields: map[string]interface{}{"customfield_1": "b", "customfield_2": map[string]interface{}{"value": "X"}, "customfield_9": 1}}
	ops, _, _ := BuildPlan(local, remote)
	if len(ops) != 2 || ops[0].Field != schema.EditableFieldPriority || ops[0].Value != "Low" ||
		ops[1].Field != schema.EditableFieldCustomFields || ops[1].Value != `{"customfield_1":"b","customfield_9":1}` {
		t.Fatalf("ops %v", ops)
	}
}
