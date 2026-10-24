package merge

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestLines(t *testing.T) {
	cases := []struct {
		name                string
		base, local, remote string
		want                string
		clean               bool
	}{
		{"identical", "a\nb\nc", "a\nb\nc", "a\nb\nc", "a\nb\nc", true},
		{"only local", "a\nb\nc", "a\nB\nc", "a\nb\nc", "a\nB\nc", true},
		{"only remote", "a\nb\nc", "a\nb\nc", "a\nb\nC", "a\nb\nC", true},
		{"disjoint hunks", "1\n2\n3\n4\n5", "L\n2\n3\n4\n5", "1\n2\n3\n4\nR", "L\n2\n3\n4\nR", true},
		{"same change both", "a\nb\nc", "a\nX\nc", "a\nX\nc", "a\nX\nc", true},
		{"overlapping", "a\nb\nc", "a\nL\nc", "a\nR\nc", "", false},
		{"adjacent lines conflict", "a\nb\nc\nd", "a\nL\nc\nd", "a\nb\nR\nd", "", false},
		{"insert and delete apart", "a\nb\nc\nd\ne", "a\nnew\nb\nc\nd\ne", "a\nb\nc\nd", "a\nnew\nb\nc\nd", true},
		{"both insert same spot differently", "a\nb", "a\nx\nb", "a\ny\nb", "", false},
		{"both delete same", "a\nb\nc", "a\nc", "a\nc", "a\nc", true},
		{"empty base", "", "x", "", "x", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, clean := Lines(tc.base, tc.local, tc.remote)
			if clean != tc.clean || (clean && got != tc.want) {
				t.Errorf("Lines = (%q, %v), want (%q, %v)", got, clean, tc.want, tc.clean)
			}
		})
	}
}

func issue(summary, desc string, labels ...string) schema.Issue {
	return schema.Issue{Summary: summary, Description: desc, Labels: labels}
}

func fieldsOf(r Result) []string {
	var out []string
	for _, c := range r.Conflicts {
		out = append(out, string(c.Field))
	}
	return out
}

func TestIssueMergeTable(t *testing.T) {
	base := issue("S", "d1\nd2\nd3\nd4\nd5", "a")

	t.Run("disjoint fields merge", func(t *testing.T) {
		local := issue("Local summary", base.Description, "a")
		remote := issue("S", base.Description, "a", "b")
		r := Issue(&base, local, remote)
		if len(r.Conflicts) != 0 {
			t.Fatalf("conflicts = %v", fieldsOf(r))
		}
		if r.Merged.Summary != "Local summary" || !reflect.DeepEqual(r.Merged.Labels, []string{"a", "b"}) {
			t.Errorf("merged = %+v", r.Merged)
		}
		if r.NewBase.Summary != "S" || !reflect.DeepEqual(r.NewBase.Labels, []string{"a", "b"}) {
			t.Errorf("new base = %+v", r.NewBase)
		}
	})

	t.Run("same field only remote changed", func(t *testing.T) {
		r := Issue(&base, issue("S", base.Description, "a"), issue("Remote", base.Description, "a"))
		if len(r.Conflicts) != 0 || r.Merged.Summary != "Remote" {
			t.Errorf("merged = %q conflicts = %v", r.Merged.Summary, fieldsOf(r))
		}
	})

	t.Run("same field only local changed", func(t *testing.T) {
		r := Issue(&base, issue("Local", base.Description, "a"), issue("S", base.Description, "a"))
		if len(r.Conflicts) != 0 || r.Merged.Summary != "Local" {
			t.Errorf("merged = %q conflicts = %v", r.Merged.Summary, fieldsOf(r))
		}
	})

	t.Run("both changed to same value", func(t *testing.T) {
		r := Issue(&base, issue("Same", base.Description, "a"), issue("Same", base.Description, "a"))
		if len(r.Conflicts) != 0 || r.Merged.Summary != "Same" {
			t.Errorf("merged = %q conflicts = %v", r.Merged.Summary, fieldsOf(r))
		}
	})

	t.Run("both changed differently conflicts and keeps local", func(t *testing.T) {
		r := Issue(&base, issue("Local", base.Description, "a"), issue("Remote", base.Description, "a"))
		if !reflect.DeepEqual(fieldsOf(r), []string{"summary"}) {
			t.Fatalf("conflicts = %v", fieldsOf(r))
		}
		c := r.Conflicts[0]
		if c.Remote != "Remote" || c.Base != "S" || r.Merged.Summary != "Local" || r.NewBase.Summary != "S" {
			t.Errorf("conflict = %+v merged = %q newBase = %q", c, r.Merged.Summary, r.NewBase.Summary)
		}
	})

	t.Run("disjoint description hunks merge", func(t *testing.T) {
		local := issue("S", "L1\nd2\nd3\nd4\nd5", "a")
		remote := issue("S", "d1\nd2\nd3\nd4\nR5", "a")
		r := Issue(&base, local, remote)
		if len(r.Conflicts) != 0 || r.Merged.Description != "L1\nd2\nd3\nd4\nR5" {
			t.Errorf("description = %q conflicts = %v", r.Merged.Description, fieldsOf(r))
		}
		if r.Merged.Sections[schema.SecDescription] != r.Merged.Description {
			t.Errorf("sections = %v", r.Merged.Sections)
		}
	})

	t.Run("overlapping description hunks conflict", func(t *testing.T) {
		local := issue("S", "d1\nLOCAL\nd3\nd4\nd5", "a")
		remote := issue("S", "d1\nREMOTE\nd3\nd4\nd5", "a")
		r := Issue(&base, local, remote)
		if !reflect.DeepEqual(fieldsOf(r), []string{"description"}) {
			t.Fatalf("conflicts = %v", fieldsOf(r))
		}
		if r.Merged.Description != local.Description || r.Conflicts[0].Remote != remote.Description || r.Conflicts[0].Base != base.Description {
			t.Errorf("merged = %q conflict = %+v", r.Merged.Description, r.Conflicts[0])
		}
		if r.NewBase.Description != base.Description {
			t.Errorf("new base description = %q, want old base", r.NewBase.Description)
		}
	})

	t.Run("no base conflicts on every differing field", func(t *testing.T) {
		local := issue("Local", "same", "a")
		remote := issue("Remote", "same", "a")
		r := Issue(nil, local, remote)
		if !reflect.DeepEqual(fieldsOf(r), []string{"summary"}) || r.Merged.Summary != "Local" {
			t.Errorf("conflicts = %v merged = %q", fieldsOf(r), r.Merged.Summary)
		}
	})

	t.Run("no base identical sides", func(t *testing.T) {
		r := Issue(nil, issue("S", "d", "a"), issue("S", "d", "a"))
		if len(r.Conflicts) != 0 {
			t.Errorf("conflicts = %v", fieldsOf(r))
		}
	})
}

func TestIssueStatusPriorityAssigneeLinks(t *testing.T) {
	who := func(s string) *string { return &s }
	base := schema.Issue{Status: "To Do", Assignee: who("ann"), Context: schema.IssueContext{Status: "To Do", Priority: "Low"}}

	local := base
	local.Status = ""
	local.Context = schema.IssueContext{Status: "In Progress", Priority: ""}
	remote := base
	remote.Assignee = who("bob")
	remote.Context = schema.IssueContext{Status: "To Do", Priority: "High"}
	remote.Status = "To Do"
	r := Issue(&base, local, remote)
	if len(r.Conflicts) != 0 {
		t.Fatalf("conflicts = %v", fieldsOf(r))
	}
	if r.Merged.Context.Status != "In Progress" || *r.Merged.Assignee != "bob" || r.Merged.Context.Priority != "High" {
		t.Errorf("merged = %+v", r.Merged)
	}

	// Status changed on both sides differently, case-insensitive compare.
	local2 := base
	local2.Status = ""
	local2.Context = schema.IssueContext{Status: "Done", Priority: "Low"}
	remote2 := base
	remote2.Status = "Blocked"
	remote2.Context = schema.IssueContext{Status: "Blocked", Priority: "Low"}
	r = Issue(&base, local2, remote2)
	if !reflect.DeepEqual(fieldsOf(r), []string{"status"}) || r.Merged.Context.Status != "Done" || r.NewBase.Status != "To Do" {
		t.Errorf("conflicts = %v merged = %+v newBase = %+v", fieldsOf(r), r.Merged, r.NewBase)
	}

	// Empty remote status never overwrites local.
	remote3 := base
	remote3.Status, remote3.Context.Status = "", ""
	r = Issue(&base, local2, remote3)
	if len(r.Conflicts) != 0 || r.Merged.Context.Status != "Done" {
		t.Errorf("conflicts = %v status = %q", fieldsOf(r), r.Merged.Context.Status)
	}

	// Linked issues: local added a link, remote unchanged.
	lbase := schema.Issue{}
	llocal := schema.Issue{LinkedIssues: []schema.LinkedIssue{{Key: "X-1", Relation: "blocks", LinkType: "Blocks"}}}
	r = Issue(&lbase, llocal, schema.Issue{})
	if len(r.Conflicts) != 0 || len(r.Merged.LinkedIssues) != 1 {
		t.Errorf("links = %+v conflicts = %v", r.Merged.LinkedIssues, fieldsOf(r))
	}
	rlinks := schema.Issue{LinkedIssues: []schema.LinkedIssue{{Key: "X-2", Relation: "blocks", LinkType: "Blocks"}}}
	r = Issue(&lbase, llocal, rlinks)
	if !reflect.DeepEqual(fieldsOf(r), []string{"linked_issues"}) || len(r.Merged.LinkedIssues) != 1 || r.Merged.LinkedIssues[0].Key != "X-1" {
		t.Errorf("links = %+v conflicts = %v", r.Merged.LinkedIssues, fieldsOf(r))
	}
}

func TestIssueCustomFields(t *testing.T) {
	cf := func(kv ...interface{}) map[string]interface{} {
		m := map[string]interface{}{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	}
	base := schema.Issue{CustomFields: cf("customfield_1", "a", "customfield_2", "b", "customfield_3", "c")}
	local := schema.Issue{CustomFields: cf("customfield_1", "A", "customfield_2", "L", "customfield_3", "c", "customfield_4", "new")}
	remote := schema.Issue{CustomFields: cf("customfield_1", "a", "customfield_2", "R", "customfield_3", "C")}
	r := Issue(&base, local, remote)
	if !reflect.DeepEqual(fieldsOf(r), []string{"custom_fields"}) {
		t.Fatalf("conflicts = %v", fieldsOf(r))
	}
	want := cf("customfield_1", "A", "customfield_2", "L", "customfield_3", "C", "customfield_4", "new")
	if !reflect.DeepEqual(r.Merged.CustomFields, want) {
		t.Errorf("merged = %v, want %v", r.Merged.CustomFields, want)
	}
	if r.Conflicts[0].Remote != `{"customfield_2":"R"}` || r.Conflicts[0].Base != `{"customfield_2":"b"}` {
		t.Errorf("conflict = %+v", r.Conflicts[0])
	}
	if r.NewBase.CustomFields["customfield_2"] != "b" || r.NewBase.CustomFields["customfield_3"] != "C" {
		t.Errorf("new base = %v", r.NewBase.CustomFields)
	}

	// Local deleted a key the remote left alone; remote-only key deleted.
	local = schema.Issue{CustomFields: cf("customfield_1", "a")}
	remote = schema.Issue{CustomFields: cf("customfield_1", "a", "customfield_2", "b")}
	r = Issue(&base, local, remote)
	if len(r.Conflicts) != 0 || r.Merged.CustomFields["customfield_2"] != nil && len(r.Merged.CustomFields) != 1 {
		t.Errorf("merged = %v conflicts = %v", r.Merged.CustomFields, fieldsOf(r))
	}

	// No base: differing key conflicts, local-only key too.
	r = Issue(nil, schema.Issue{CustomFields: cf("customfield_1", "x")}, schema.Issue{})
	if !reflect.DeepEqual(fieldsOf(r), []string{"custom_fields"}) || r.Conflicts[0].Base != "{}" {
		t.Errorf("conflicts = %+v", r.Conflicts)
	}
}

func TestLocalSections(t *testing.T) {
	local := schema.Issue{Sections: map[schema.FixedSectionName]string{
		schema.SecNotes:              "my notes",
		schema.SecAcceptanceCriteria: "",
		schema.SecComments:           "stale",
		schema.SecDescription:        "old",
	}}
	got := localSections(local, "")
	if len(got) != 1 || got[schema.SecNotes] != "my notes" {
		t.Errorf("sections = %v", got)
	}
	got = localSections(local, "desc")
	if got[schema.SecDescription] != "desc" || strings.Contains(got[schema.SecNotes], "stale") {
		t.Errorf("sections = %v", got)
	}
}

func TestDiffHunksEmpty(t *testing.T) {
	if h := diffHunks([]string{"a"}, []string{"a"}, 0); h != nil {
		t.Errorf("hunks = %v", h)
	}
}
