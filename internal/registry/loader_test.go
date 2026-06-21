package registry

import (
	"os"
	"path/filepath"
	"testing"
)

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
