package registry

import (
	"errors"
	"testing"
)

func TestParseTypedRef_valid_user(t *testing.T) {
	refType, key, err := ParseTypedRef("user:jesper")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refType != "user" {
		t.Errorf("refType = %q, want %q", refType, "user")
	}
	if key != "jesper" {
		t.Errorf("key = %q, want %q", key, "jesper")
	}
}

func TestParseTypedRef_valid_project(t *testing.T) {
	refType, key, err := ParseTypedRef("project:ABC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refType != "project" {
		t.Errorf("refType = %q, want %q", refType, "project")
	}
	if key != "ABC" {
		t.Errorf("key = %q, want %q", key, "ABC")
	}
}

func TestParseTypedRef_valid_with_colon_in_key(t *testing.T) {
	// Account IDs contain colons: "user:712020:abcd"
	refType, key, err := ParseTypedRef("user:712020:abcd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refType != "user" {
		t.Errorf("refType = %q, want %q", refType, "user")
	}
	if key != "712020:abcd" {
		t.Errorf("key = %q, want %q", key, "712020:abcd")
	}
}

func TestParseTypedRef_empty(t *testing.T) {
	_, _, err := ParseTypedRef("")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
	if !errors.Is(err, errMissingRef) {
		t.Errorf("errors.Is(err, errMissingRef) = false, want true")
	}
}

func TestParseTypedRef_no_colon(t *testing.T) {
	_, _, err := ParseTypedRef("userjesper")
	if err == nil {
		t.Fatal("expected error for ref without colon")
	}
	if !errors.Is(err, errMissingRef) {
		t.Errorf("errors.Is(err, errMissingRef) = false, want true")
	}
}

func TestResolveUser_found(t *testing.T) {
	users := map[string]User{
		"user:jesper": {
			AccountID:   "712020:abcd",
			DisplayName: "Jesper Ronn",
			Email:       "jesper@example.com",
			Active:      true,
		},
		"user:bob": {
			AccountID:   "712020:efgh",
			DisplayName: "Bob Smith",
			Email:       "bob@example.com",
			Active:      false,
		},
	}

	// Exact match by account_id
	accountID, found := ResolveUser("user:jesper", users)
	if !found {
		t.Fatal("expected found for user:jesper")
	}
	if accountID != "712020:abcd" {
		t.Errorf("accountID = %q, want %q", accountID, "712020:abcd")
	}

	// Another user
	accountID, found = ResolveUser("user:bob", users)
	if !found {
		t.Fatal("expected found for user:bob")
	}
	if accountID != "712020:efgh" {
		t.Errorf("accountID = %q, want %q", accountID, "712020:efgh")
	}
}

func TestResolveUser_not_found(t *testing.T) {
	users := map[string]User{
		"user:jesper": {
			AccountID:   "712020:abcd",
			DisplayName: "Jesper Ronn",
		},
	}

	_, found := ResolveUser("user:missing", users)
	if found {
		t.Error("expected not found for user:missing")
	}
}

func TestResolveUser_empty_ref(t *testing.T) {
	users := map[string]User{
		"user:jesper": {AccountID: "712020:abcd"},
	}

	_, found := ResolveUser("", users)
	if found {
		t.Error("expected not found for empty ref")
	}
}

func TestResolveUser_nil_map(t *testing.T) {
	_, found := ResolveUser("user:jesper", nil)
	if found {
		t.Error("expected not found for nil map")
	}
}

func TestResolveProject_found(t *testing.T) {
	projects := map[string]Project{
		"project:ABC": {
			Key:     "ABC",
			Name:    "A Big Project",
			ID:      "10000",
			Lead:    "712020:abcd",
			ProjectType: "software",
		},
		"project:XYZ": {
			Key:     "XYZ",
			Name:    "XYZ Project",
			ID:      "20000",
			ProjectType: "business",
		},
	}

	// Exact match by key
	key, found := ResolveProject("project:ABC", projects)
	if !found {
		t.Fatal("expected found for project:ABC")
	}
	if key != "ABC" {
		t.Errorf("key = %q, want %q", key, "ABC")
	}

	// Another project
	key, found = ResolveProject("project:XYZ", projects)
	if !found {
		t.Fatal("expected found for project:XYZ")
	}
	if key != "XYZ" {
		t.Errorf("key = %q, want %q", key, "XYZ")
	}
}

func TestResolveProject_not_found(t *testing.T) {
	projects := map[string]Project{
		"project:ABC": {Key: "ABC", Name: "A Big Project"},
	}

	_, found := ResolveProject("project:MISSING", projects)
	if found {
		t.Error("expected not found for project:MISSING")
	}
}

func TestResolveProject_empty_ref(t *testing.T) {
	projects := map[string]Project{
		"project:ABC": {Key: "ABC"},
	}

	_, found := ResolveProject("", projects)
	if found {
		t.Error("expected not found for empty ref")
	}
}

func TestResolveProject_nil_map(t *testing.T) {
	_, found := ResolveProject("project:ABC", nil)
	if found {
		t.Error("expected not found for nil map")
	}
}

func TestResolveError_Error(t *testing.T) {
	re := &ResolveError{
		RefType: "user",
		Ref:     "user:missing",
		Code:    "missing_ref",
		Msg:     "no user found for user:missing",
	}
	want := "registry: missing_ref: no user found for user:missing"
	got := re.Error()
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestResolveStatus_found(t *testing.T) {
	statuses := map[string]Status{
		"status:in-progress": {
			Name:      "In Progress",
			Category:  "InProgress",
			Description: "Work is currently being done.",
		},
		"status:done": {
			Name:     "Done",
			Category: "Done",
		},
	}

	name, found := ResolveStatus("status:in-progress", statuses)
	if !found {
		t.Fatal("expected found for status:in-progress")
	}
	if name != "In Progress" {
		t.Errorf("name = %q, want %q", name, "In Progress")
	}

	name, found = ResolveStatus("status:done", statuses)
	if !found {
		t.Fatal("expected found for status:done")
	}
	if name != "Done" {
		t.Errorf("name = %q, want %q", name, "Done")
	}
}

func TestResolveStatus_not_found(t *testing.T) {
	statuses := map[string]Status{
		"status:in-progress": {Name: "In Progress"},
	}

	_, found := ResolveStatus("status:missing", statuses)
	if found {
		t.Error("expected not found for status:missing")
	}
}

func TestResolveStatus_empty_ref(t *testing.T) {
	statuses := map[string]Status{
		"status:in-progress": {Name: "In Progress"},
	}

	_, found := ResolveStatus("", statuses)
	if found {
		t.Error("expected not found for empty ref")
	}
}

func TestResolveStatus_nil_map(t *testing.T) {
	_, found := ResolveStatus("status:in-progress", nil)
	if found {
		t.Error("expected not found for nil map")
	}
}

func TestResolveSprint_found(t *testing.T) {
	sprints := map[string]Sprint{
		"sprint:12345": {
			ID:    12345,
			Name:  "Sprint 24",
			State: "active",
		},
		"sprint:12346": {
			ID:    12346,
			Name:  "Sprint 25",
			State: "closed",
		},
	}

	id, found := ResolveSprint("sprint:12345", sprints)
	if !found {
		t.Fatal("expected found for sprint:12345")
	}
	if id != "12345" {
		t.Errorf("id = %q, want %q", id, "12345")
	}

	id, found = ResolveSprint("sprint:12346", sprints)
	if !found {
		t.Fatal("expected found for sprint:12346")
	}
	if id != "12346" {
		t.Errorf("id = %q, want %q", id, "12346")
	}
}

func TestResolveSprint_not_found(t *testing.T) {
	sprints := map[string]Sprint{
		"sprint:12345": {ID: 12345, Name: "Sprint 24"},
	}

	_, found := ResolveSprint("sprint:99999", sprints)
	if found {
		t.Error("expected not found for sprint:99999")
	}
}

func TestResolveSprint_empty_ref(t *testing.T) {
	sprints := map[string]Sprint{
		"sprint:12345": {ID: 12345},
	}

	_, found := ResolveSprint("", sprints)
	if found {
		t.Error("expected not found for empty ref")
	}
}

func TestResolveSprint_nil_map(t *testing.T) {
	_, found := ResolveSprint("sprint:12345", nil)
	if found {
		t.Error("expected not found for nil map")
	}
}

func TestResolveSprint_zero_id(t *testing.T) {
	sprints := map[string]Sprint{
		"sprint:0": {ID: 0, Name: "Sprint 0"},
	}

	id, found := ResolveSprint("sprint:0", sprints)
	if !found {
		t.Fatal("expected found for sprint:0")
	}
	if id != "0" {
		t.Errorf("id = %q, want %q", id, "0")
	}
}

func TestResolveFixVersion_found(t *testing.T) {
	fixVersions := map[string]FixVersion{
		"fix-version:1.4.0": {
			Name:        "1.4.0",
			Description: "Minor release with bug fixes",
			Archived:    false,
			Released:    true,
		},
		"fix-version:2.0.0": {
			Name:     "2.0.0",
			Released: false,
		},
	}

	name, found := ResolveFixVersion("fix-version:1.4.0", fixVersions)
	if !found {
		t.Fatal("expected found for fix-version:1.4.0")
	}
	if name != "1.4.0" {
		t.Errorf("name = %q, want %q", name, "1.4.0")
	}

	name, found = ResolveFixVersion("fix-version:2.0.0", fixVersions)
	if !found {
		t.Fatal("expected found for fix-version:2.0.0")
	}
	if name != "2.0.0" {
		t.Errorf("name = %q, want %q", name, "2.0.0")
	}
}

func TestResolveFixVersion_not_found(t *testing.T) {
	fixVersions := map[string]FixVersion{
		"fix-version:1.4.0": {Name: "1.4.0"},
	}

	_, found := ResolveFixVersion("fix-version:3.0.0", fixVersions)
	if found {
		t.Error("expected not found for fix-version:3.0.0")
	}
}

func TestResolveFixVersion_empty_ref(t *testing.T) {
	fixVersions := map[string]FixVersion{
		"fix-version:1.4.0": {Name: "1.4.0"},
	}

	_, found := ResolveFixVersion("", fixVersions)
	if found {
		t.Error("expected not found for empty ref")
	}
}

func TestResolveFixVersion_nil_map(t *testing.T) {
	_, found := ResolveFixVersion("fix-version:1.4.0", nil)
	if found {
		t.Error("expected not found for nil map")
	}
}

func TestIsResolveError(t *testing.T) {
	re := &ResolveError{Code: "missing_ref"}

	tests := []struct {
		name string
		err  error
		code string
		want bool
	}{
		{
			name: "matching code",
			err:  re,
			code: "missing_ref",
			want: true,
		},
		{
			name: "different code",
			err:  re,
			code: "ambiguous_ref",
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			code: "missing_ref",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsResolveError(tt.err, tt.code)
			if got != tt.want {
				t.Errorf("IsResolveError(%v, %q) = %v, want %v", tt.err, tt.code, got, tt.want)
			}
		})
	}
}
