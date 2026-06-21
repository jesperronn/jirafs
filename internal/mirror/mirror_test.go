package mirror

import (
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestImportReason_IsValid(t *testing.T) {
	for _, r := range ValidImportReasons {
		if !IsValidImportReason(r) {
			t.Errorf("IsValidImportReason(%q) = false, want true", r)
		}
	}
	if IsValidImportReason("bogus") {
		t.Error("IsValidImportReason(\"bogus\") = true, want false")
	}
	var zero ImportReason
	if IsValidImportReason(zero) {
		t.Error("IsValidImportReason(\"\") = true, want false")
	}
}

func TestImportedIssue_IsZero(t *testing.T) {
	var zero ImportedIssue
	if !zero.IsZero() {
		t.Error("zero ImportedIssue should be zero")
	}
	nonZero := ImportedIssue{Key: "PROJ-123", Reason: ImportReasonManual}
	if nonZero.IsZero() {
		t.Error("non-zero ImportedIssue should not be zero")
	}
	partial := ImportedIssue{Key: "PROJ-123"}
	if partial.IsZero() {
		t.Error("ImportedIssue with only Key set should not be zero")
	}
}

func TestImportedIssue_String(t *testing.T) {
	imp := ImportedIssue{Key: "PROJ-123", Reason: ImportReasonManual}
	got := imp.String()
	want := "PROJ-123 (manual)"
	if got != want {
		t.Errorf("ImportedIssue.String() = %q, want %q", got, want)
	}
}

func TestMirror_IsZero(t *testing.T) {
	var zero Mirror
	if !zero.IsZero() {
		t.Error("zero Mirror should be zero")
	}
	nonZero := Mirror{Project: schema.TypedRef{Type: schema.RefProject, Value: "ABC"}}
	if nonZero.IsZero() {
		t.Error("Mirror with project set should not be zero")
	}
	nonZero2 := Mirror{Issues: []ImportedIssue{{Key: "PROJ-123", Reason: ImportReasonManual}}}
	if nonZero2.IsZero() {
		t.Error("Mirror with issues set should not be zero")
	}
}

func TestMirror_HasIssue(t *testing.T) {
	m := Mirror{
		Project: schema.TypedRef{Type: schema.RefProject, Value: "ABC"},
		Issues: []ImportedIssue{
			{Key: "PROJ-123", Reason: ImportReasonManual},
			{Key: "PROJ-456", Reason: ImportReasonDependency},
		},
	}
	if !m.HasIssue("PROJ-123") {
		t.Error("mirror should have PROJ-123")
	}
	if !m.HasIssue("PROJ-456") {
		t.Error("mirror should have PROJ-456")
	}
	if m.HasIssue("PROJ-789") {
		t.Error("mirror should not have PROJ-789")
	}
}

func TestMirror_HasIssue_empty(t *testing.T) {
	var m Mirror
	if m.HasIssue("PROJ-123") {
		t.Error("empty mirror should not have any issues")
	}
	m.Issues = nil
	if m.HasIssue("PROJ-123") {
		t.Error("mirror with nil issues should not have any issues")
	}
}

func TestMirror_ImportReasonFor(t *testing.T) {
	m := Mirror{
		Issues: []ImportedIssue{
			{Key: "PROJ-123", Reason: ImportReasonManual},
			{Key: "PROJ-456", Reason: ImportReasonDependency},
		},
	}
	if got := m.ImportReasonFor("PROJ-123"); got != ImportReasonManual {
		t.Errorf("ImportReasonFor(PROJ-123) = %q, want %q", got, ImportReasonManual)
	}
	if got := m.ImportReasonFor("PROJ-456"); got != ImportReasonDependency {
		t.Errorf("ImportReasonFor(PROJ-456) = %q, want %q", got, ImportReasonDependency)
	}
	if got := m.ImportReasonFor("PROJ-789"); got != "" {
		t.Errorf("ImportReasonFor(PROJ-789) = %q, want \"\"", got)
	}
}

func TestMirror_ImportReasonFor_empty(t *testing.T) {
	var m Mirror
	if got := m.ImportReasonFor("PROJ-123"); got != "" {
		t.Errorf("empty mirror ImportReasonFor(PROJ-123) = %q, want \"\"", got)
	}
}

func TestIsValidScopeType(t *testing.T) {
	for _, tpe := range ValidScopeTypes {
		if !IsValidScopeType(tpe) {
			t.Errorf("IsValidScopeType(%q) = false, want true", tpe)
		}
	}
	if IsValidScopeType("bogus") {
		t.Error("IsValidScopeType(\"bogus\") = true, want false")
	}
	var zero ScopeType
	if IsValidScopeType(zero) {
		t.Error("IsValidScopeType(\"\") = true, want false")
	}
}

func TestScope_IsZero(t *testing.T) {
	var zero Scope
	if !zero.IsZero() {
		t.Error("zero Scope should be zero")
	}
	partial := Scope{Name: "active"}
	if partial.IsZero() {
		t.Error("Scope with only Name set should not be zero")
	}
	nonZero := Scope{Name: "active", Type: ScopeTypeJQL, Target: "status=Active"}
	if nonZero.IsZero() {
		t.Error("non-zero Scope should not be zero")
	}
}

func TestScopeMember_IsZero(t *testing.T) {
	var zero ScopeMember
	if !zero.IsZero() {
		t.Error("zero ScopeMember should be zero")
	}
	partial := ScopeMember{Key: "PROJ-123"}
	if partial.IsZero() {
		t.Error("ScopeMember with only Key set should not be zero")
	}
	nonZero := ScopeMember{Key: "PROJ-123", Scope: "active"}
	if nonZero.IsZero() {
		t.Error("non-zero ScopeMember should not be zero")
	}
}

func TestScopeMember_String(t *testing.T) {
	mem := ScopeMember{Key: "PROJ-123", Scope: "active"}
	got := mem.String()
	want := "PROJ-123 (@active)"
	if got != want {
		t.Errorf("ScopeMember.String() = %q, want %q", got, want)
	}
}

func TestMirror_HasScope(t *testing.T) {
	m := Mirror{
		Scopes: []Scope{
			{Name: "active", Type: ScopeTypeJQL, Target: "status=Active"},
			{Name: "epics", Type: ScopeTypeComponent, Target: "Epic"},
		},
	}
	if !m.HasScope("active") {
		t.Error("mirror should have active scope")
	}
	if !m.HasScope("epics") {
		t.Error("mirror should have epics scope")
	}
	if m.HasScope("inactive") {
		t.Error("mirror should not have inactive scope")
	}
}

func TestMirror_HasScope_empty(t *testing.T) {
	var m Mirror
	if m.HasScope("active") {
		t.Error("empty mirror should not have any scopes")
	}
	m.Scopes = nil
	if m.HasScope("active") {
		t.Error("mirror with nil scopes should not have any scopes")
	}
}

func TestMirror_ScopeFor(t *testing.T) {
	m := Mirror{
		Scopes: []Scope{
			{Name: "active", Type: ScopeTypeJQL, Target: "status=Active"},
		},
	}
	got := m.ScopeFor("active")
	if got.Name != "active" || got.Type != ScopeTypeJQL || got.Target != "status=Active" {
		t.Errorf("ScopeFor(active) = %+v, want {Name:active Type:jql Target:status=Active}", got)
	}
	got = m.ScopeFor("inactive")
	if !got.IsZero() {
		t.Errorf("ScopeFor(inactive) = %+v, want zero Scope", got)
	}
}

func TestMirror_ScopeTypeFor(t *testing.T) {
	m := Mirror{
		Scopes: []Scope{
			{Name: "active", Type: ScopeTypeJQL, Target: "status=Active"},
		},
	}
	if got := m.ScopeTypeFor("active"); got != ScopeTypeJQL {
		t.Errorf("ScopeTypeFor(active) = %q, want %q", got, ScopeTypeJQL)
	}
	if got := m.ScopeTypeFor("inactive"); got != "" {
		t.Errorf("ScopeTypeFor(inactive) = %q, want \"\"", got)
	}
}

func TestMirror_HasScopeMember(t *testing.T) {
	m := Mirror{
		ScopeMembers: []ScopeMember{
			{Key: "PROJ-123", Scope: "active"},
			{Key: "PROJ-456", Scope: "epics"},
		},
	}
	if !m.HasScopeMember("PROJ-123") {
		t.Error("mirror should have PROJ-123 as scope member")
	}
	if !m.HasScopeMember("PROJ-456") {
		t.Error("mirror should have PROJ-456 as scope member")
	}
	if m.HasScopeMember("PROJ-789") {
		t.Error("mirror should not have PROJ-789 as scope member")
	}
}

func TestMirror_HasScopeMember_empty(t *testing.T) {
	var m Mirror
	if m.HasScopeMember("PROJ-123") {
		t.Error("empty mirror should not have any scope members")
	}
	m.ScopeMembers = nil
	if m.HasScopeMember("PROJ-123") {
		t.Error("mirror with nil scope members should not have any")
	}
}

func TestMirror_ScopeMemberFor(t *testing.T) {
	m := Mirror{
		ScopeMembers: []ScopeMember{
			{Key: "PROJ-123", Scope: "active"},
			{Key: "PROJ-456", Scope: "epics"},
		},
	}
	if got := m.ScopeMemberFor("PROJ-123"); got != "active" {
		t.Errorf("ScopeMemberFor(PROJ-123) = %q, want %q", got, "active")
	}
	if got := m.ScopeMemberFor("PROJ-456"); got != "epics" {
		t.Errorf("ScopeMemberFor(PROJ-456) = %q, want %q", got, "epics")
	}
	if got := m.ScopeMemberFor("PROJ-789"); got != "" {
		t.Errorf("ScopeMemberFor(PROJ-789) = %q, want \"\"", got)
	}
}

func TestMirror_AddScope(t *testing.T) {
	m := Mirror{}
	if !m.AddScope(Scope{Name: "active", Type: ScopeTypeJQL, Target: "status=Active"}) {
		t.Error("AddScope should return true for new scope")
	}
	if len(m.Scopes) != 1 {
		t.Errorf("expected 1 scope, got %d", len(m.Scopes))
	}
	// Adding same name again should fail
	if m.AddScope(Scope{Name: "active", Type: ScopeTypeJQL, Target: "status=Active"}) {
		t.Error("AddScope should return false for duplicate scope")
	}
	if len(m.Scopes) != 1 {
		t.Errorf("expected 1 scope after duplicate add, got %d", len(m.Scopes))
	}
	// Zero scope should fail
	if m.AddScope(Scope{}) {
		t.Error("AddScope should return false for zero scope")
	}
}

func TestMirror_AddScopeMember(t *testing.T) {
	m := Mirror{}
	if !m.AddScopeMember(ScopeMember{Key: "PROJ-123", Scope: "active"}) {
		t.Error("AddScopeMember should return true for new member")
	}
	if len(m.ScopeMembers) != 1 {
		t.Errorf("expected 1 scope member, got %d", len(m.ScopeMembers))
	}
	// Adding same key again should fail
	if m.AddScopeMember(ScopeMember{Key: "PROJ-123", Scope: "active"}) {
		t.Error("AddScopeMember should return false for duplicate member")
	}
	if len(m.ScopeMembers) != 1 {
		t.Errorf("expected 1 scope member after duplicate add, got %d", len(m.ScopeMembers))
	}
	// Zero member should fail
	if m.AddScopeMember(ScopeMember{}) {
		t.Error("AddScopeMember should return false for zero member")
	}
}

func TestMirror_ExplicitAndScopeMembers(t *testing.T) {
	// Verify that explicit imports and scope memberships are tracked separately.
	m := Mirror{
		Project: schema.TypedRef{Type: schema.RefProject, Value: "ABC"},
		Issues: []ImportedIssue{
			{Key: "PROJ-100", Reason: ImportReasonManual},
		},
		Scopes: []Scope{
			{Name: "active", Type: ScopeTypeJQL, Target: "status=Active"},
		},
		ScopeMembers: []ScopeMember{
			{Key: "PROJ-200", Scope: "active"},
		},
	}
	// Explicit import is not a scope member
	if m.HasScopeMember("PROJ-100") {
		t.Error("PROJ-100 should not be a scope member")
	}
	// Scope member is not an explicit import
	if m.HasIssue("PROJ-200") {
		t.Error("PROJ-200 should not be an explicit import")
	}
	// Both can coexist
	if !m.HasIssue("PROJ-100") {
		t.Error("PROJ-100 should be an explicit import")
	}
	if !m.HasScopeMember("PROJ-200") {
		t.Error("PROJ-200 should be a scope member")
	}
}

func TestIsValidResolvedStatus(t *testing.T) {
	for _, s := range ValidResolvedStatuses {
		if !IsValidResolvedStatus(s) {
			t.Errorf("IsValidResolvedStatus(%q) = false, want true", s)
		}
	}
	if IsValidResolvedStatus("bogus") {
		t.Error("IsValidResolvedStatus(\"bogus\") = true, want false")
	}
	var zero ResolvedStatus
	if IsValidResolvedStatus(zero) {
		t.Error("IsValidResolvedStatus(\"\") = true, want false")
	}
}

func TestArchiveEligible_IsZero(t *testing.T) {
	var zero ArchiveEligible
	if !zero.IsZero() {
		t.Error("zero ArchiveEligible should be zero")
	}
	partial := ArchiveEligible{Key: "PROJ-123"}
	if partial.IsZero() {
		t.Error("ArchiveEligible with only Key set should not be zero")
	}
	nonZero := ArchiveEligible{Key: "PROJ-123", ResolvedStatus: ResolvedStatusResolved}
	if nonZero.IsZero() {
		t.Error("non-zero ArchiveEligible should not be zero")
	}
}

func TestArchiveEligible_String(t *testing.T) {
	a := ArchiveEligible{Key: "PROJ-123", ResolvedStatus: ResolvedStatusResolved}
	got := a.String()
	want := "PROJ-123 (resolved)"
	if got != want {
		t.Errorf("ArchiveEligible.String() = %q, want %q", got, want)
	}
}

func TestMirror_IsArchiveEligible(t *testing.T) {
	m := Mirror{
		Project: schema.TypedRef{Type: schema.RefProject, Value: "ABC"},
		Issues: []ImportedIssue{
			{Key: "PROJ-100", Reason: ImportReasonManual},
		},
		ScopeMembers: []ScopeMember{
			{Key: "PROJ-200", Scope: "active"},
		},
	}
	// Out-of-scope + resolved = eligible
	if !m.IsArchiveEligible("PROJ-300", ResolvedStatusResolved) {
		t.Error("out-of-scope resolved issue should be archive-eligible")
	}
	// Explicitly imported + resolved = not eligible
	if m.IsArchiveEligible("PROJ-100", ResolvedStatusResolved) {
		t.Error("explicitly imported issue should not be archive-eligible")
	}
	// Scope member + resolved = not eligible
	if m.IsArchiveEligible("PROJ-200", ResolvedStatusResolved) {
		t.Error("scope member should not be archive-eligible")
	}
	// Out-of-scope + open = not eligible
	if m.IsArchiveEligible("PROJ-300", ResolvedStatusOpen) {
		t.Error("out-of-scope open issue should not be archive-eligible")
	}
}

func TestMirror_IsArchiveEligible_empty(t *testing.T) {
	var m Mirror
	// Empty mirror: out-of-scope + resolved = eligible
	if !m.IsArchiveEligible("PROJ-123", ResolvedStatusResolved) {
		t.Error("empty mirror: out-of-scope resolved issue should be eligible")
	}
}
