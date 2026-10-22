package export

import (
	"reflect"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestNormalizeCustomFieldValues(t *testing.T) {
	var i schema.Issue
	fields := map[string]interface{}{
		"customfield_1": "text", "customfield_2": map[string]interface{}{"value": "V", "id": "9"},
		"customfield_3": nil, "customfield_4": 2.0, "customfield_5": 2.5,
		"customfield_6": []interface{}{}, "customfield_7": map[string]interface{}{"name": "bob", "key": "k"},
		"customfield_8": map[string]interface{}{"other": 1}, "customfield_9": true,
		"customfield_10": []interface{}{map[string]interface{}{"value": "A"}, nil},
	}
	ids := []string{"customfield_1", "customfield_2", "customfield_3", "customfield_4", "customfield_5", "customfield_6", "customfield_7", "customfield_8", "customfield_9", "customfield_10", "customfield_11"}
	NormalizeCustomFieldValues(&i, nil, ids, nil)
	NormalizeCustomFieldValues(&i, fields, nil, nil)
	NormalizeCustomFieldValues(&i, fields, ids, map[string]string{"customfield_1": "One", "customfield_11": "Eleven"})
	want := map[string]interface{}{
		"customfield_1": "text", "customfield_2": map[string]interface{}{"value": "V"},
		"customfield_4": int64(2), "customfield_5": 2.5,
		"customfield_7": map[string]interface{}{"name": "bob"}, "customfield_8": map[string]interface{}{"other": 1},
		"customfield_9": true, "customfield_10": []interface{}{map[string]interface{}{"value": "A"}},
	}
	if !reflect.DeepEqual(i.CustomFields, want) {
		t.Errorf("got %#v", i.CustomFields)
	}
	if len(i.Context.CustomFieldNames) != 2 {
		t.Errorf("names %v", i.Context.CustomFieldNames)
	}
}

func TestNormalizeContextProject(t *testing.T) {
	var i schema.Issue
	NormalizeContext(&i, map[string]interface{}{"project": map[string]interface{}{"key": "ABC"}}, "")
	if i.Identity.Project.String() != "project:ABC" {
		t.Errorf("%v", i.Identity.Project)
	}
}
