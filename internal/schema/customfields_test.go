package schema

import (
	"reflect"
	"strings"
	"testing"
)

const cfDoc = `---
type: 'story'
project: 'project:ABC'
state: 'draft'
summary: 'S'
custom_fields:
  customfield_12601:
    value: Team A
  customfield_10002: 5
  customfield_10003:
  - id: "7"
  - id: "8"
  customfield_10004: plain text
---
`

func TestCustomFieldsParseRenderRoundTrip(t *testing.T) {
	i, pe := ParseIssue(cfDoc)
	if pe != nil {
		t.Fatal(pe)
	}
	want := map[string]interface{}{
		"customfield_12601": map[string]interface{}{"value": "Team A"},
		"customfield_10002": 5,
		"customfield_10003": []interface{}{map[string]interface{}{"id": "7"}, map[string]interface{}{"id": "8"}},
		"customfield_10004": "plain text",
	}
	if !reflect.DeepEqual(i.CustomFields, want) {
		t.Fatalf("got %#v", i.CustomFields)
	}
	out := RenderIssue(i)
	if !strings.Contains(out, "custom_fields:\n  customfield_10002: 5\n") {
		t.Errorf("render:\n%s", out)
	}
	j, pe := ParseIssue(out)
	if pe != nil || !reflect.DeepEqual(j.CustomFields, want) {
		t.Fatalf("round trip: %v %#v", pe, j.CustomFields)
	}
	if RenderIssue(j) != out {
		t.Error("render not stable")
	}
}

func TestCustomFieldsInvalid(t *testing.T) {
	for _, fm := range []string{"custom_fields:\n  foo: 1\n", "custom_fields: [1]\n"} {
		if _, pe := ParseIssue("---\ntype: story\n" + fm + "---\n"); pe == nil {
			t.Errorf("expected error for %q", fm)
		}
	}
	if m, err := NormalizeCustomFields(map[string]interface{}{}); m != nil || err != nil {
		t.Error("empty map should be nil")
	}
}

func TestMergeCustomFieldsPrecedence(t *testing.T) {
	d := map[string]interface{}{"customfield_1": "def", "customfield_2": "def2"}
	o := map[string]interface{}{"customfield_1": "own"}
	got := MergeCustomFields(d, o)
	if got["customfield_1"] != "own" || got["customfield_2"] != "def2" || len(got) != 2 {
		t.Errorf("got %v", got)
	}
	if MergeCustomFields(nil, nil) != nil {
		t.Error("want nil")
	}
	if d["customfield_1"] != "def" {
		t.Error("defaults mutated")
	}
}
