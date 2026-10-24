package sync

import (
	"testing"

	"github.com/jirafs/jirafs/internal/plan"
	"github.com/jirafs/jirafs/internal/schema"
)

func iss(summary, desc string) schema.Issue {
	return schema.Issue{Summary: summary, Description: desc}
}

func TestDetectConflicts(t *testing.T) {
	base := iss("S", "D")
	cases := []struct {
		name          string
		base          *schema.Issue
		local, remote schema.Issue
		want          []schema.EditableField
	}{
		{"only local", &base, iss("S", "D2"), iss("S", "D"), nil},
		{"only remote", &base, iss("S", "D"), iss("S", "D3"), nil},
		{"both same value", &base, iss("S", "D2"), iss("S", "D2"), nil},
		{"both different", &base, iss("S", "D2"), iss("S", "D3"), []schema.EditableField{"description"}},
		{"two fields one conflict", &base, iss("S2", "D2"), iss("S", "D3"), []schema.EditableField{"description"}},
		{"two fields two conflicts", &base, iss("S2", "D2"), iss("S3", "D3"), []schema.EditableField{"summary", "description"}},
		{"missing base differs", nil, iss("S", "D2"), iss("S", "D3"), []schema.EditableField{"description"}},
		{"missing base equal", nil, iss("S", "D"), iss("S", "D"), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectConflicts(tc.base, tc.local, tc.remote)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want fields %v", got, tc.want)
			}
			for i, c := range got {
				if c.Field != tc.want[i] || c.Type != schema.ConflictBothEdited {
					t.Errorf("conflict %d = %v, want field %s", i, c, tc.want[i])
				}
			}
		})
	}
}

func TestDetectConflicts_valuesAndEmptyMetadata(t *testing.T) {
	base := iss("S", "D")
	local, remote := iss("S", "mine"), iss("S", "theirs")
	// Empty version and hash on both sides must not skip the check.
	got := DetectConflicts(&base, local, remote)
	if len(got) != 1 || got[0].LocalValue != "mine" || got[0].RemoteValue != "theirs" || got[0].Field != schema.EditableFieldDescription {
		t.Fatalf("got %v", got)
	}
	// Differing versions alone are not a conflict when no field collides.
	local, remote = iss("S", "D2"), iss("S", "D")
	local.RemoteMetadata.RemoteVersion, remote.RemoteMetadata.RemoteVersion = "1", "2"
	if got := DetectConflicts(&base, local, remote); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestDetectConflicts_otherFields(t *testing.T) {
	a, b, c := "a", "b", "c"
	base := schema.Issue{Labels: []string{"x"}, FixVersions: []string{"1"}, Assignee: &a, Status: "To Do", Sprint: "1",
		Context: schema.IssueContext{Priority: "Low"}, CustomFields: map[string]interface{}{"customfield_1": "o", "customfield_2": "o"}}
	local := schema.Issue{Labels: []string{"y"}, FixVersions: []string{"2"}, Assignee: &b, Status: "Done", Sprint: "2",
		Context: schema.IssueContext{Priority: "High"}, CustomFields: map[string]interface{}{"customfield_1": "l", "customfield_2": "l"}}
	remote := schema.Issue{Labels: []string{"z"}, FixVersions: []string{"3"}, Assignee: &c, Status: "In Review", Sprint: "3",
		Context: schema.IssueContext{Priority: "Mid"}, CustomFields: map[string]interface{}{"customfield_1": "r", "customfield_2": "o"}}
	got := DetectConflicts(&base, local, remote)
	want := []schema.EditableField{"status", "labels", "fix_versions", "assignee", "sprint", "priority", "custom_fields"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i, g := range got {
		if g.Field != want[i] {
			t.Errorf("%d: %s, want %s", i, g.Field, want[i])
		}
	}
	last := got[len(got)-1]
	if last.LocalValue != `{"customfield_1":"l"}` || last.RemoteValue != `{"customfield_1":"r"}` {
		t.Errorf("custom conflict = %v", last)
	}
	// Missing base: status/priority unset locally are skipped, same status ignores case.
	local2 := schema.Issue{Status: "to do", CustomFields: map[string]interface{}{"customfield_1": "l"}}
	remote2 := schema.Issue{Status: "To Do", Context: schema.IssueContext{Priority: "Mid"}, CustomFields: map[string]interface{}{"customfield_1": "r"}}
	got = DetectConflicts(nil, local2, remote2)
	if len(got) != 1 || got[0].Field != schema.EditableFieldCustomFields {
		t.Fatalf("got %v", got)
	}
	if got := DetectConflicts(nil, schema.Issue{}, schema.Issue{Status: "X"}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if got := DetectConflicts(nil, schema.Issue{Status: "X"}, schema.Issue{}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestSync_descriptionCollisionReportsDescription(t *testing.T) {
	base := iss("S", "D")
	local, remote := iss("S", "mine"), iss("S", "theirs")
	ops, _, _ := plan.BuildPlanWithBase(&base, local, remote)
	res := Sync(&base, local, remote, ops, t.TempDir())
	if len(res.Conflicts) != 1 || res.Conflicts[0].Field != schema.EditableFieldDescription {
		t.Fatalf("conflicts = %v", res.Conflicts)
	}
	if res.Remote.Description != "theirs" {
		t.Fatalf("remote mutated: %q", res.Remote.Description)
	}
}

func TestBuildPlanWithBase_skipsRemoteOnlyChanges(t *testing.T) {
	base := schema.Issue{Summary: "S", Description: "D", CustomFields: map[string]interface{}{"customfield_1": "o", "customfield_2": "o"}}
	local := schema.Issue{Summary: "S", Description: "D2", CustomFields: map[string]interface{}{"customfield_1": "o", "customfield_2": "n"}}
	remote := schema.Issue{Summary: "S remote", Description: "D", CustomFields: map[string]interface{}{"customfield_1": "r", "customfield_2": "o"}}
	ops, _, err := plan.BuildPlanWithBase(&base, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 || ops[0].Field != "description" || ops[1].Field != "custom_fields" || ops[1].Value != `{"customfield_2":"n"}` {
		t.Fatalf("ops = %v", ops)
	}
	res := Sync(&base, local, remote, ops, t.TempDir())
	if len(res.Conflicts) != 0 || res.Remote.Summary != "S remote" || res.Remote.Description != "D2" {
		t.Fatalf("res = %+v", res)
	}
	// Custom fields untouched locally yield no op.
	local.CustomFields["customfield_2"] = "o"
	ops, _, _ = plan.BuildPlanWithBase(&base, local, remote)
	if len(ops) != 1 {
		t.Fatalf("ops = %v", ops)
	}
	// Nil base is the plain plan.
	plain, _, _ := plan.BuildPlanWithBase(nil, local, remote)
	want, _, _ := plan.BuildPlan(local, remote)
	if len(plain) != len(want) {
		t.Fatalf("nil base plan %v != %v", plain, want)
	}
}
