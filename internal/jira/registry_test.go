package jira

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jirafs/jirafs/internal/registry"
)

func TestRefreshRegistries_success(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetStatuses([]StatusEntry{
		{Name: "To Do", Category: "New", StatusKey: "todo"},
		{Name: "In Progress", Category: "InProgress", StatusKey: "in-progress"},
		{Name: "Done", Category: "Done", StatusKey: "done"},
	})
	fake.SetSprints("PROJ", []SprintEntry{
		{ID: 100, Name: "Sprint 1", State: "active", StartDate: "2024-01-01T00:00:00Z", EndDate: "2024-01-14T00:00:00Z"},
		{ID: 101, Name: "Sprint 2", State: "closed"},
	})
	fake.SetFixVersions("PROJ", []FixVersionEntry{
		{Name: "1.0.0", Description: "First release", Archived: false, Released: true},
		{Name: "1.1.0", Archived: false, Released: false},
	})

	dir := t.TempDir()
	ctx := context.Background()

	if err := RefreshRegistries(ctx, fake, "PROJ", dir); err != nil {
		t.Fatalf("RefreshRegistries: %v", err)
	}

	// Verify statuses.yaml
	statuses, err := registry.LoadStatuses(dir)
	if err != nil {
		t.Fatalf("LoadStatuses: %v", err)
	}
	if len(statuses) != 3 {
		t.Errorf("expected 3 statuses, got %d", len(statuses))
	}
	if s, ok := statuses["status:todo"]; !ok || s.Name != "To Do" {
		t.Errorf("missing or wrong status:todo: %+v", s)
	}
	if s, ok := statuses["status:in-progress"]; !ok || s.Name != "In Progress" {
		t.Errorf("missing or wrong status:in-progress: %+v", s)
	}
	if s, ok := statuses["status:done"]; !ok || s.Name != "Done" {
		t.Errorf("missing or wrong status:done: %+v", s)
	}

	// Verify sprints.yaml
	sprints, err := registry.LoadSprints(dir)
	if err != nil {
		t.Fatalf("LoadSprints: %v", err)
	}
	if len(sprints) != 2 {
		t.Errorf("expected 2 sprints, got %d", len(sprints))
	}
	if sp, ok := sprints["sprint:100"]; !ok || sp.Name != "Sprint 1" {
		t.Errorf("missing or wrong sprint:100: %+v", sp)
	}
	if sp, ok := sprints["sprint:101"]; !ok || sp.Name != "Sprint 2" {
		t.Errorf("missing or wrong sprint:101: %+v", sp)
	}

	// Verify fix_versions.yaml
	versions, err := registry.LoadFixVersions(dir)
	if err != nil {
		t.Fatalf("LoadFixVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("expected 2 fix versions, got %d", len(versions))
	}
	if fv, ok := versions["fix-version:1.0.0"]; !ok || !fv.Released {
		t.Errorf("missing or wrong fix-version:1.0.0: %+v", fv)
	}
}

func TestRefreshRegistries_statusKeyFallback(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetStatuses([]StatusEntry{
		{Name: "In Review", Category: "InProgress"}, // No StatusKey
	})
	fake.SetSprints("PROJ", []SprintEntry{})
	fake.SetFixVersions("PROJ", []FixVersionEntry{})

	dir := t.TempDir()
	ctx := context.Background()

	if err := RefreshRegistries(ctx, fake, "PROJ", dir); err != nil {
		t.Fatalf("RefreshRegistries: %v", err)
	}

	statuses, err := registry.LoadStatuses(dir)
	if err != nil {
		t.Fatalf("LoadStatuses: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	// Should be keyed by normalized name: "in-review"
	key := "status:in-review"
	if _, ok := statuses[key]; !ok {
		t.Errorf("expected key %q, got keys: %v", key, mapKeys(statuses))
	}
}

func TestRefreshRegistries_emptyStatuses(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetStatuses([]StatusEntry{}) // empty but set
	fake.SetSprints("PROJ", []SprintEntry{})
	fake.SetFixVersions("PROJ", []FixVersionEntry{})

	dir := t.TempDir()
	ctx := context.Background()

	if err := RefreshRegistries(ctx, fake, "PROJ", dir); err != nil {
		t.Fatalf("RefreshRegistries: %v", err)
	}

	statuses, err := registry.LoadStatuses(dir)
	if err != nil {
		t.Fatalf("LoadStatuses: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(statuses))
	}
}

func TestRefreshRegistries_fetchStatusesError(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetSprints("PROJ", []SprintEntry{})
	fake.SetFixVersions("PROJ", []FixVersionEntry{})
	// Don't set statuses → will return not_found

	dir := t.TempDir()
	ctx := context.Background()

	err := RefreshRegistries(ctx, fake, "PROJ", dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got: %v", err)
	}

	// Verify sprints.yaml was NOT written (statuses failed first)
	_, err = os.Stat(filepath.Join(dir, registry.StatusesFile))
	if err == nil {
		t.Error("statuses.yaml should not exist after error")
	}
}

func TestRefreshRegistries_fetchSprintsError(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetStatuses([]StatusEntry{
		{Name: "Open", Category: "New", StatusKey: "open"},
	})
	// Don't set sprints for PROJ → will return not_found
	fake.SetFixVersions("PROJ", []FixVersionEntry{})

	dir := t.TempDir()
	ctx := context.Background()

	err := RefreshRegistries(ctx, fake, "PROJ", dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got: %v", err)
	}

	// Verify statuses.yaml WAS written (statuses succeeded)
	_, err = os.Stat(filepath.Join(dir, registry.StatusesFile))
	if err != nil {
		t.Errorf("statuses.yaml should exist: %v", err)
	}
}

func TestRefreshRegistries_fetchFixVersionsError(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetStatuses([]StatusEntry{
		{Name: "Open", Category: "New", StatusKey: "open"},
	})
	fake.SetSprints("PROJ", []SprintEntry{})
	// Don't set fix versions for PROJ → will return not_found

	dir := t.TempDir()
	ctx := context.Background()

	err := RefreshRegistries(ctx, fake, "PROJ", dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got: %v", err)
	}

	// Verify statuses.yaml and sprints.yaml were written
	_, err = os.Stat(filepath.Join(dir, registry.StatusesFile))
	if err != nil {
		t.Errorf("statuses.yaml should exist: %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, registry.SprintsFile))
	if err != nil {
		t.Errorf("sprints.yaml should exist: %v", err)
	}
}

func TestRefreshRegistries_sprintDates(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetStatuses([]StatusEntry{})
	fake.SetSprints("PROJ", []SprintEntry{
		{ID: 200, Name: "Sprint with dates", State: "active",
			StartDate:    "2024-06-01T08:00:00Z",
			EndDate:      "2024-06-15T18:00:00Z",
			CompleteDate: "2024-06-14T12:00:00Z",
		},
		{ID: 201, Name: "Sprint without dates", State: "future"},
	})
	fake.SetFixVersions("PROJ", []FixVersionEntry{})

	dir := t.TempDir()
	ctx := context.Background()

	if err := RefreshRegistries(ctx, fake, "PROJ", dir); err != nil {
		t.Fatalf("RefreshRegistries: %v", err)
	}

	sprints, err := registry.LoadSprints(dir)
	if err != nil {
		t.Fatalf("LoadSprints: %v", err)
	}

	sp := sprints["sprint:200"]
	if sp.StartDate == nil {
		t.Error("expected StartDate to be set")
	} else if sp.StartDate.Format("2006-01-02") != "2024-06-01" {
		t.Errorf("StartDate = %v, want 2024-06-01", *sp.StartDate)
	}
	if sp.EndDate == nil {
		t.Error("expected EndDate to be set")
	}
	if sp.CompleteDate == nil {
		t.Error("expected CompleteDate to be set")
	}

	sp2 := sprints["sprint:201"]
	if sp2.StartDate != nil || sp2.EndDate != nil || sp2.CompleteDate != nil {
		t.Error("expected all dates to be nil for sprint without dates")
	}
}

func TestRefreshRegistries_mkdirAll(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetStatuses([]StatusEntry{})
	fake.SetSprints("PROJ", []SprintEntry{})
	fake.SetFixVersions("PROJ", []FixVersionEntry{})

	// Create a deep nested directory that doesn't exist
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	ctx := context.Background()

	if err := RefreshRegistries(ctx, fake, "PROJ", dir); err != nil {
		t.Fatalf("RefreshRegistries: %v", err)
	}

	// Verify directories were created
	_, err := os.Stat(dir)
	if err != nil {
		t.Errorf("directory %s should exist: %v", dir, err)
	}
}

func TestRefreshRegistries_fixVersionFields(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetStatuses([]StatusEntry{})
	fake.SetSprints("PROJ", []SprintEntry{})
	fake.SetFixVersions("PROJ", []FixVersionEntry{
		{Name: "2.0.0", Description: "Major release", Archived: true, Released: true},
		{Name: "2.1.0", Archived: false, Released: false},
		{Name: "1.0.0", Archived: true, Released: true}, // archived
	})

	dir := t.TempDir()
	ctx := context.Background()

	if err := RefreshRegistries(ctx, fake, "PROJ", dir); err != nil {
		t.Fatalf("RefreshRegistries: %v", err)
	}

	versions, err := registry.LoadFixVersions(dir)
	if err != nil {
		t.Fatalf("LoadFixVersions: %v", err)
	}

	// Check archived version
	fv := versions["fix-version:1.0.0"]
	if !fv.Archived {
		t.Error("expected archived version to have Archived=true")
	}
	if !fv.Released {
		t.Error("expected archived version to have Released=true")
	}

	// Check non-archived version
	fv2 := versions["fix-version:2.1.0"]
	if fv2.Archived {
		t.Error("expected non-archived version to have Archived=false")
	}
	if fv2.Released {
		t.Error("expected non-released version to have Released=false")
	}
}

func TestRefreshRegistries_sprintInvalidDates(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetStatuses([]StatusEntry{})
	fake.SetSprints("PROJ", []SprintEntry{
		{ID: 300, Name: "Sprint with invalid dates", State: "active",
			StartDate:    "not-a-date",
			EndDate:      "also-not-a-date",
			CompleteDate: "neither-is-this",
		},
	})
	fake.SetFixVersions("PROJ", []FixVersionEntry{})

	dir := t.TempDir()
	ctx := context.Background()

	if err := RefreshRegistries(ctx, fake, "PROJ", dir); err != nil {
		t.Fatalf("RefreshRegistries: %v", err)
	}

	sprints, err := registry.LoadSprints(dir)
	if err != nil {
		t.Fatalf("LoadSprints: %v", err)
	}

	sp := sprints["sprint:300"]
	if sp.StartDate != nil {
		t.Error("expected StartDate to be nil for invalid date")
	}
	if sp.EndDate != nil {
		t.Error("expected EndDate to be nil for invalid date")
	}
	if sp.CompleteDate != nil {
		t.Error("expected CompleteDate to be nil for invalid date")
	}
}

// mapKeys returns the keys of a map for testing purposes.
func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
