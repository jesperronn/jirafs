package registry

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// nilUnwrapperError is an error that implements Unwrap() but returns nil.
// Used to test the nil-unwrapping branch in IsRegistryError.
type nilUnwrapperError struct {
	msg string
}

func (e *nilUnwrapperError) Error() string {
	return e.msg
}

func (e *nilUnwrapperError) Unwrap() error {
	return nil
}

// multiLevelWrapperError wraps another error, enabling multi-level unwrapping.
// Used to test the `continue` branch in IsRegistryError and IsResolveError.
type multiLevelWrapperError struct {
	msg   string
	inner error
}

func (e *multiLevelWrapperError) Error() string {
	return e.msg
}

func (e *multiLevelWrapperError) Unwrap() error {
	return e.inner
}

func TestLoadUsersFile_notFound(t *testing.T) {
	_, err := LoadUsersFile("/nonexistent/path/users.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !IsRegistryError(err, "file_not_found") {
		t.Errorf("expected file_not_found error, got: %v", err)
	}
}

func TestLoadUsersFile_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, UsersFile)
	if err := os.WriteFile(path, []byte("[invalid yaml content:"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUsersFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !IsRegistryError(err, "yaml_parse") {
		t.Errorf("expected yaml_parse error, got: %v", err)
	}
}

func TestLoadUsersFile_missingEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, UsersFile)
	if err := os.WriteFile(path, []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUsersFile(path)
	if err == nil {
		t.Fatal("expected error for missing entries key")
	}
	if !IsRegistryError(err, "missing_entries") {
		t.Errorf("expected missing_entries error, got: %v", err)
	}
}

func TestLoadUsersFile_emptyEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, UsersFile)
	content := `entries: {}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadUsersFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries == nil {
		t.Fatal("expected non-nil entries map")
	}
	if len(entries) != 0 {
		t.Errorf("expected empty map, got %d entries", len(entries))
	}
}

func TestLoadUsersFile_valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, UsersFile)
	content := `entries:
  "user:jesper":
    account_id: "712020:abcd"
    display_name: "Jesper Ronn"
    email: "jesper@example.com"
    active: true
  "user:bob":
    account_id: "712020:efgh"
    display_name: "Bob Smith"
    email: "bob@example.com"
    active: false
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadUsersFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	jesper := entries["user:jesper"]
	if jesper.AccountID != "712020:abcd" {
		t.Errorf("AccountID = %q, want %q", jesper.AccountID, "712020:abcd")
	}
	if jesper.DisplayName != "Jesper Ronn" {
		t.Errorf("DisplayName = %q, want %q", jesper.DisplayName, "Jesper Ronn")
	}
	if jesper.Email != "jesper@example.com" {
		t.Errorf("Email = %q, want %q", jesper.Email, "jesper@example.com")
	}
	if !jesper.Active {
		t.Error("Active should be true")
	}

	bob := entries["user:bob"]
	if bob.DisplayName != "Bob Smith" {
		t.Errorf("DisplayName = %q, want %q", bob.DisplayName, "Bob Smith")
	}
	if bob.Active {
		t.Error("Active should be false")
	}
}

func TestLoadProjectsFile_notFound(t *testing.T) {
	_, err := LoadProjectsFile("/nonexistent/path/projects.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !IsRegistryError(err, "file_not_found") {
		t.Errorf("expected file_not_found error, got: %v", err)
	}
}

func TestLoadProjectsFile_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ProjectsFile)
	if err := os.WriteFile(path, []byte("[invalid yaml content:"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProjectsFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !IsRegistryError(err, "yaml_parse") {
		t.Errorf("expected yaml_parse error, got: %v", err)
	}
}

func TestLoadProjectsFile_missingEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ProjectsFile)
	if err := os.WriteFile(path, []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProjectsFile(path)
	if err == nil {
		t.Fatal("expected error for missing entries key")
	}
	if !IsRegistryError(err, "missing_entries") {
		t.Errorf("expected missing_entries error, got: %v", err)
	}
}

func TestLoadProjectsFile_valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ProjectsFile)
	content := `entries:
  "project:abc":
    key: "ABC"
    name: "A Big Project"
    id: "10000"
    avatar: "https://example.com/avatar.png"
    lead: "712020:abcd"
    project_type: "software"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadProjectsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	abc := entries["project:abc"]
	if abc.Key != "ABC" {
		t.Errorf("Key = %q, want %q", abc.Key, "ABC")
	}
	if abc.Name != "A Big Project" {
		t.Errorf("Name = %q, want %q", abc.Name, "A Big Project")
	}
	if abc.ID != "10000" {
		t.Errorf("ID = %q, want %q", abc.ID, "10000")
	}
	if abc.Avatar != "https://example.com/avatar.png" {
		t.Errorf("Avatar = %q, want %q", abc.Avatar, "https://example.com/avatar.png")
	}
	if abc.Lead != "712020:abcd" {
		t.Errorf("Lead = %q, want %q", abc.Lead, "712020:abcd")
	}
	if abc.ProjectType != "software" {
		t.Errorf("ProjectType = %q, want %q", abc.ProjectType, "software")
	}
}

func TestLoadUsers_helper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, UsersFile)
	content := `entries:
  "user:alice":
    account_id: "712020:xyz"
    display_name: "Alice"
    email: "alice@example.com"
    active: true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadUsers(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if _, ok := entries["user:alice"]; !ok {
		t.Error("expected user:alice in entries")
	}
}

func TestLoadProjects_helper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ProjectsFile)
	content := `entries:
  "project:xyz":
    key: "XYZ"
    name: "XYZ Project"
    id: "20000"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadProjects(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if _, ok := entries["project:xyz"]; !ok {
		t.Error("expected project:xyz in entries")
	}
}

func TestRegistryError_Error(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		code  string
		msg   string
		want  string
	}{
		{
			name: "with file",
			file: "/path/users.yaml",
			code: "file_not_found",
			msg:  "users registry file not found",
			want: "registry: file_not_found (/path/users.yaml): users registry file not found",
		},
		{
			name: "without file",
			file: "",
			code: "yaml_parse",
			msg:  "invalid YAML",
			want: "registry: yaml_parse: invalid YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewRegistryError(tt.file, tt.code, tt.msg)
			got := e.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadStatusesFile_notFound(t *testing.T) {
	_, err := LoadStatusesFile("/nonexistent/path/statuses.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !IsRegistryError(err, "file_not_found") {
		t.Errorf("expected file_not_found error, got: %v", err)
	}
}

func TestLoadStatusesFile_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, StatusesFile)
	if err := os.WriteFile(path, []byte("[invalid yaml content:"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadStatusesFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !IsRegistryError(err, "yaml_parse") {
		t.Errorf("expected yaml_parse error, got: %v", err)
	}
}

func TestLoadStatusesFile_missingEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, StatusesFile)
	if err := os.WriteFile(path, []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadStatusesFile(path)
	if err == nil {
		t.Fatal("expected error for missing entries key")
	}
	if !IsRegistryError(err, "missing_entries") {
		t.Errorf("expected missing_entries error, got: %v", err)
	}
}

func TestLoadStatusesFile_emptyEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, StatusesFile)
	content := `entries: {}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadStatusesFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries == nil {
		t.Fatal("expected non-nil entries map")
	}
	if len(entries) != 0 {
		t.Errorf("expected empty map, got %d entries", len(entries))
	}
}

func TestLoadStatusesFile_valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, StatusesFile)
	content := `entries:
  "status:in-progress":
    name: "In Progress"
    category: "InProgress"
    description: "Work is currently being done."
  "status:done":
    name: "Done"
    category: "Done"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadStatusesFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	ip := entries["status:in-progress"]
	if ip.Name != "In Progress" {
		t.Errorf("Name = %q, want %q", ip.Name, "In Progress")
	}
	if ip.Category != "InProgress" {
		t.Errorf("Category = %q, want %q", ip.Category, "InProgress")
	}
	if ip.Description != "Work is currently being done." {
		t.Errorf("Description = %q, want %q", ip.Description, "Work is currently being done.")
	}

	done := entries["status:done"]
	if done.Name != "Done" {
		t.Errorf("Name = %q, want %q", done.Name, "Done")
	}
	if done.Category != "Done" {
		t.Errorf("Category = %q, want %q", done.Category, "Done")
	}
}

func TestLoadSprintsFile_notFound(t *testing.T) {
	_, err := LoadSprintsFile("/nonexistent/path/sprints.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !IsRegistryError(err, "file_not_found") {
		t.Errorf("expected file_not_found error, got: %v", err)
	}
}

func TestLoadSprintsFile_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SprintsFile)
	if err := os.WriteFile(path, []byte("[invalid yaml content:"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadSprintsFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !IsRegistryError(err, "yaml_parse") {
		t.Errorf("expected yaml_parse error, got: %v", err)
	}
}

func TestLoadSprintsFile_missingEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SprintsFile)
	if err := os.WriteFile(path, []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadSprintsFile(path)
	if err == nil {
		t.Fatal("expected error for missing entries key")
	}
	if !IsRegistryError(err, "missing_entries") {
		t.Errorf("expected missing_entries error, got: %v", err)
	}
}

func TestLoadSprintsFile_valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SprintsFile)
	content := `entries:
  "sprint:12345":
    id: 12345
    name: "Sprint 24"
    state: "active"
    start_date: "2024-06-01T00:00:00Z"
    end_date: "2024-06-14T00:00:00Z"
  "sprint:12346":
    id: 12346
    name: "Sprint 25"
    state: "closed"
    complete_date: "2024-06-15T00:00:00Z"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadSprintsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	s1 := entries["sprint:12345"]
	if s1.ID != 12345 {
		t.Errorf("ID = %d, want %d", s1.ID, 12345)
	}
	if s1.Name != "Sprint 24" {
		t.Errorf("Name = %q, want %q", s1.Name, "Sprint 24")
	}
	if s1.State != "active" {
		t.Errorf("State = %q, want %q", s1.State, "active")
	}

	s2 := entries["sprint:12346"]
	if s2.Name != "Sprint 25" {
		t.Errorf("Name = %q, want %q", s2.Name, "Sprint 25")
	}
	if s2.State != "closed" {
		t.Errorf("State = %q, want %q", s2.State, "closed")
	}
}

func TestLoadFixVersionsFile_notFound(t *testing.T) {
	_, err := LoadFixVersionsFile("/nonexistent/path/fix_versions.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !IsRegistryError(err, "file_not_found") {
		t.Errorf("expected file_not_found error, got: %v", err)
	}
}

func TestLoadFixVersionsFile_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FixVersionsFile)
	if err := os.WriteFile(path, []byte("[invalid yaml content:"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFixVersionsFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !IsRegistryError(err, "yaml_parse") {
		t.Errorf("expected yaml_parse error, got: %v", err)
	}
}

func TestLoadFixVersionsFile_missingEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FixVersionsFile)
	if err := os.WriteFile(path, []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFixVersionsFile(path)
	if err == nil {
		t.Fatal("expected error for missing entries key")
	}
	if !IsRegistryError(err, "missing_entries") {
		t.Errorf("expected missing_entries error, got: %v", err)
	}
}

func TestLoadFixVersionsFile_valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FixVersionsFile)
	content := `entries:
  "fix-version:1.4.0":
    name: "1.4.0"
    description: "Minor release with bug fixes"
    archived: false
    released: true
  "fix-version:1.5.0":
    name: "1.5.0"
    description: "Feature release"
    archived: false
    released: false
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadFixVersionsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	fv1 := entries["fix-version:1.4.0"]
	if fv1.Name != "1.4.0" {
		t.Errorf("Name = %q, want %q", fv1.Name, "1.4.0")
	}
	if fv1.Description != "Minor release with bug fixes" {
		t.Errorf("Description = %q, want %q", fv1.Description, "Minor release with bug fixes")
	}
	if fv1.Archived {
		t.Error("Archived should be false")
	}
	if !fv1.Released {
		t.Error("Released should be true")
	}

	fv2 := entries["fix-version:1.5.0"]
	if fv2.Name != "1.5.0" {
		t.Errorf("Name = %q, want %q", fv2.Name, "1.5.0")
	}
	if fv2.Archived {
		t.Error("Archived should be false")
	}
	if fv2.Released {
		t.Error("Released should be false")
	}
}

func TestLoadStatuses_helper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, StatusesFile)
	content := `entries:
  "status:todo":
    name: "To Do"
    category: "ToDos"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadStatuses(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if _, ok := entries["status:todo"]; !ok {
		t.Error("expected status:todo in entries")
	}
}

func TestLoadSprints_helper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SprintsFile)
	content := `entries:
  "sprint:99":
    id: 99
    name: "Test Sprint"
    state: "active"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadSprints(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if _, ok := entries["sprint:99"]; !ok {
		t.Error("expected sprint:99 in entries")
	}
}

func TestLoadFixVersions_helper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FixVersionsFile)
	content := `entries:
  "fix-version:2.0":
    name: "2.0"
    description: "Major release"
    released: true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadFixVersions(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if _, ok := entries["fix-version:2.0"]; !ok {
		t.Error("expected fix-version:2.0 in entries")
	}
}

func TestIsRegistryError(t *testing.T) {
	re := NewRegistryError("/path/users.yaml", "file_not_found", "not found")

	tests := []struct {
		name string
		err  error
		code string
		want bool
	}{
		{
			name: "matching code",
			err:  re,
			code: "file_not_found",
			want: true,
		},
		{
			name: "different code",
			err:  re,
			code: "yaml_parse",
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			code: "file_not_found",
			want: false,
		},
		{
			name: "wrapped error",
			err:  os.ErrNotExist,
			code: "file_not_found",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRegistryError(tt.err, tt.code)
			if got != tt.want {
				t.Errorf("IsRegistryError(%v, %q) = %v, want %v", tt.err, tt.code, got, tt.want)
			}
		})
	}
}

func TestIsRegistryError_nilUnwrapper(t *testing.T) {
	nilUnwrapper := &nilUnwrapperError{msg: "wrapped but nil"}
	if IsRegistryError(nilUnwrapper, "some_code") {
		t.Error("expected false for nil-unwrapping error")
	}
}

func TestIsRegistryError_multiLevelUnwrap(t *testing.T) {
	// An error that wraps another error, enabling the `continue` branch
	// in the for loop of IsRegistryError.
	innerErr := errors.New("inner error")
	wrapped := &multiLevelWrapperError{msg: "outer", inner: innerErr}
	if IsRegistryError(wrapped, "some_code") {
		t.Error("expected false for multi-level wrapped error")
	}
}

func TestLoadUsersFile_permissionDenied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, UsersFile)
	content := `entries:
  "user:alice":
    account_id: "712020:xyz"
`
	if err := os.WriteFile(path, []byte(content), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUsersFile(path)
	if err == nil {
		t.Fatal("expected error for permission denied")
	}
	if !IsRegistryError(err, "file_read") {
		t.Errorf("expected file_read error, got: %v", err)
	}
}

func TestLoadProjectsFile_permissionDenied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ProjectsFile)
	content := `entries:
  "project:abc":
    key: "ABC"
`
	if err := os.WriteFile(path, []byte(content), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProjectsFile(path)
	if err == nil {
		t.Fatal("expected error for permission denied")
	}
	if !IsRegistryError(err, "file_read") {
		t.Errorf("expected file_read error, got: %v", err)
	}
}

func TestLoadStatusesFile_permissionDenied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, StatusesFile)
	content := `entries:
  "status:todo":
    name: "To Do"
`
	if err := os.WriteFile(path, []byte(content), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := LoadStatusesFile(path)
	if err == nil {
		t.Fatal("expected error for permission denied")
	}
	if !IsRegistryError(err, "file_read") {
		t.Errorf("expected file_read error, got: %v", err)
	}
}

func TestLoadSprintsFile_permissionDenied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SprintsFile)
	content := `entries:
  "sprint:99":
    id: 99
`
	if err := os.WriteFile(path, []byte(content), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := LoadSprintsFile(path)
	if err == nil {
		t.Fatal("expected error for permission denied")
	}
	if !IsRegistryError(err, "file_read") {
		t.Errorf("expected file_read error, got: %v", err)
	}
}

func TestLoadFixVersionsFile_permissionDenied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FixVersionsFile)
	content := `entries:
  "fix-version:2.0":
    name: "2.0"
`
	if err := os.WriteFile(path, []byte(content), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFixVersionsFile(path)
	if err == nil {
		t.Fatal("expected error for permission denied")
	}
	if !IsRegistryError(err, "file_read") {
		t.Errorf("expected file_read error, got: %v", err)
	}
}
