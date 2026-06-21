package registry

import (
	"errors"
	"strings"
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

func TestErrMissingRef_includes_type_and_lookup_value(t *testing.T) {
	err := ErrMissingRef("user", "user:missing")

	if err.Code != "missing_ref" {
		t.Errorf("Code = %q, want %q", err.Code, "missing_ref")
	}
	if err.RefType != "user" {
		t.Errorf("RefType = %q, want %q", err.RefType, "user")
	}
	if err.Ref != "user:missing" {
		t.Errorf("Ref = %q, want %q", err.Ref, "user:missing")
	}

	// Verify the error message includes both type and lookup value.
	have := err.Error()
	if !strings.Contains(have, "user") {
		t.Errorf("Error() = %q does not contain type %q", have, "user")
	}
	if !strings.Contains(have, "user:missing") {
		t.Errorf("Error() = %q does not contain lookup value %q", have, "user:missing")
	}
}

func TestErrMissingRef_project(t *testing.T) {
	err := ErrMissingRef("project", "project:XYZ")

	if err.RefType != "project" {
		t.Errorf("RefType = %q, want %q", err.RefType, "project")
	}
	if err.Ref != "project:XYZ" {
		t.Errorf("Ref = %q, want %q", err.Ref, "project:XYZ")
	}

	want := "registry: missing_ref: no project found for project:XYZ"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestErrMissingRef_status(t *testing.T) {
	err := ErrMissingRef("status", "status:in-review")

	want := "registry: missing_ref: no status found for status:in-review"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestErrMissingRef_sprint(t *testing.T) {
	err := ErrMissingRef("sprint", "sprint:99999")

	want := "registry: missing_ref: no sprint found for sprint:99999"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestErrMissingRef_fix_version(t *testing.T) {
	err := ErrMissingRef("fix-version", "fix-version:3.0.0")

	want := "registry: missing_ref: no fix-version found for fix-version:3.0.0"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestErrAmbiguousRef_includes_candidates(t *testing.T) {
	candidates := []string{"Jesper Ronn", "Jesper Smith"}
	err := ErrAmbiguousRef("user", "jesper", candidates)

	if err.Code != "ambiguous_ref" {
		t.Errorf("Code = %q, want %q", err.Code, "ambiguous_ref")
	}
	if err.RefType != "user" {
		t.Errorf("RefType = %q, want %q", err.RefType, "user")
	}
	if err.Ref != "jesper" {
		t.Errorf("Ref = %q, want %q", err.Ref, "jesper")
	}
	if len(err.Candidates) != 2 {
		t.Errorf("Candidates = %v, want 2 candidates", err.Candidates)
	}
	if err.Candidates[0] != "Jesper Ronn" {
		t.Errorf("Candidates[0] = %q, want %q", err.Candidates[0], "Jesper Ronn")
	}
	if err.Candidates[1] != "Jesper Smith" {
		t.Errorf("Candidates[1] = %q, want %q", err.Candidates[1], "Jesper Smith")
	}

	// Verify the error message includes candidate count and lookup value.
	have := err.Error()
	if !strings.Contains(have, "2") {
		t.Errorf("Error() = %q does not contain candidate count", have)
	}
	if !strings.Contains(have, "jesper") {
		t.Errorf("Error() = %q does not contain lookup value", have)
	}
}

func TestErrAmbiguousRef_single_candidate(t *testing.T) {
	err := ErrAmbiguousRef("project", "ABC", []string{"A Big Project"})

	if err.Code != "ambiguous_ref" {
		t.Errorf("Code = %q, want %q", err.Code, "ambiguous_ref")
	}
	want := "registry: ambiguous_ref: 1 project candidates for ABC"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestErrAmbiguousRef_zero_candidates(t *testing.T) {
	err := ErrAmbiguousRef("user", "nobody", []string{})

	if err.Code != "ambiguous_ref" {
		t.Errorf("Code = %q, want %q", err.Code, "ambiguous_ref")
	}
	want := "registry: ambiguous_ref: 0 user candidates for nobody"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestIsResolveError_ambiguous_ref(t *testing.T) {
	err := ErrAmbiguousRef("user", "jesper", []string{"Jesper Ronn"})

	if !IsResolveError(err, "ambiguous_ref") {
		t.Error("IsResolveError(err, \"ambiguous_ref\") = false, want true")
	}
	if IsResolveError(err, "missing_ref") {
		t.Error("IsResolveError(err, \"missing_ref\") = true, want false")
	}
}

func TestResolveUserAmbiguous_exact_display_name_match(t *testing.T) {
	users := map[string]User{
		"user:jesper":  {AccountID: "712020:abcd", DisplayName: "Jesper Ronn"},
		"user:bob":     {AccountID: "712020:efgh", DisplayName: "Bob Smith"},
		"user:jenny":   {AccountID: "712020:ijkl", DisplayName: "Jenny Lee"},
	}

	// Exact match returns the single candidate
	accountID, found, err := ResolveUserAmbiguous("Jesper Ronn", users)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found for Jesper Ronn")
	}
	if accountID != "712020:abcd" {
		t.Errorf("accountID = %q, want %q", accountID, "712020:abcd")
	}
}

func TestResolveUserAmbiguous_partial_display_name_match(t *testing.T) {
	users := map[string]User{
		"user:jesper": {AccountID: "712020:abcd", DisplayName: "Jesper Ronn"},
		"user:bob":    {AccountID: "712020:efgh", DisplayName: "Bob Smith"},
	}

	// Partial match on "Jesper" finds exactly one user
	accountID, found, err := ResolveUserAmbiguous("Jesper", users)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found for Jesper")
	}
	if accountID != "712020:abcd" {
		t.Errorf("accountID = %q, want %q", accountID, "712020:abcd")
	}
}

func TestResolveUserAmbiguous_ambiguous_matches(t *testing.T) {
	users := map[string]User{
		"user:jesper":  {AccountID: "712020:abcd", DisplayName: "Jesper Ronn"},
		"user:jesper2": {AccountID: "712020:efgh", DisplayName: "Jesper Smith"},
		"user:bob":     {AccountID: "712020:ijkl", DisplayName: "Bob Smith"},
	}

	_, found, err := ResolveUserAmbiguous("Jesper", users)
	if err == nil {
		t.Fatal("expected error for ambiguous lookup")
	}
	if found {
		t.Error("expected not found for ambiguous lookup")
	}
	if !IsResolveError(err, "ambiguous_ref") {
		t.Errorf("expected ambiguous_ref error, got: %v", err)
	}
	re := err.(*ResolveError)
	if len(re.Candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(re.Candidates))
	}
	have := make(map[string]bool)
	for _, c := range re.Candidates {
		have[c] = true
	}
	if !have["Jesper Ronn"] {
		t.Errorf("candidates missing Jesper Ronn: %v", re.Candidates)
	}
	if !have["Jesper Smith"] {
		t.Errorf("candidates missing Jesper Smith: %v", re.Candidates)
	}
}

func TestResolveUserAmbiguous_no_match(t *testing.T) {
	users := map[string]User{
		"user:jesper": {AccountID: "712020:abcd", DisplayName: "Jesper Ronn"},
		"user:bob":    {AccountID: "712020:efgh", DisplayName: "Bob Smith"},
	}

	accountID, found, err := ResolveUserAmbiguous("Nobody", users)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected not found for Nobody")
	}
	if accountID != "" {
		t.Errorf("accountID = %q, want empty", accountID)
	}
}

func TestResolveUserAmbiguous_empty_lookup(t *testing.T) {
	users := map[string]User{
		"user:jesper": {AccountID: "712020:abcd", DisplayName: "Jesper Ronn"},
	}

	accountID, found, err := ResolveUserAmbiguous("", users)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected not found for empty lookup")
	}
	if accountID != "" {
		t.Errorf("accountID = %q, want empty", accountID)
	}
}

func TestResolveUserAmbiguous_nil_map(t *testing.T) {
	accountID, found, err := ResolveUserAmbiguous("Jesper", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected not found for nil map")
	}
	if accountID != "" {
		t.Errorf("accountID = %q, want empty", accountID)
	}
}

func TestResolveUserAmbiguous_case_insensitive(t *testing.T) {
	users := map[string]User{
		"user:jesper": {AccountID: "712020:abcd", DisplayName: "Jesper Ronn"},
	}

	// Lowercase lookup should match uppercase display name
	accountID, found, err := ResolveUserAmbiguous("jesper", users)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found for lowercase 'jesper'")
	}
	if accountID != "712020:abcd" {
		t.Errorf("accountID = %q, want %q", accountID, "712020:abcd")
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
