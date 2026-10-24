package mirror

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func baseIssue(summary string) *schema.Issue {
	return &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-1"},
		Summary:  summary,
	}
}

func TestSaveAndLoadBase(t *testing.T) {
	dir := t.TempDir()
	if err := SaveBase(dir, baseIssue("First")); err != nil {
		t.Fatal(err)
	}
	got, ok := LoadBase(dir, "PROJ-1")
	if !ok || got.Summary != "First" {
		t.Fatalf("LoadBase = %+v, %v", got, ok)
	}
	if err := SaveBase(dir, baseIssue("Second")); err != nil {
		t.Fatal(err)
	}
	got, _ = LoadBase(dir, "PROJ-1")
	if got.Summary != "Second" {
		t.Fatalf("base not replaced: %q", got.Summary)
	}
	entries, _ := os.ReadDir(BaseDir(dir))
	if len(entries) != 1 || entries[0].Name() != "PROJ-1.md" {
		t.Fatalf("unexpected base dir entries: %v", entries)
	}
	data, _ := os.ReadFile(BasePath(dir, "PROJ-1"))
	if strings.Contains(string(data), "synced_hash") || strings.Contains(string(data), "sync_time") {
		t.Fatalf("volatile fields in base:\n%s", data)
	}
}

func TestSaveBaseNoMirrorDir(t *testing.T) {
	if err := SaveBase("", baseIssue("x")); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadBase("", "PROJ-1"); ok {
		t.Fatal("empty mirror dir must have no base")
	}
}

func TestLoadBaseMissing(t *testing.T) {
	if _, ok := LoadBase(t.TempDir(), "PROJ-9"); ok {
		t.Fatal("missing base returned true")
	}
}

func TestLoadBaseCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(BaseDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(BasePath(dir, "PROJ-1"), []byte("---\n: [bad\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadBase(dir, "PROJ-1"); ok {
		t.Fatal("corrupt base returned true")
	}
}

func TestSaveBaseErrors(t *testing.T) {
	dir := t.TempDir()
	// .jirafs is a file, so MkdirAll fails.
	if err := os.WriteFile(filepath.Join(dir, ".jirafs"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveBase(dir, baseIssue("x")); err == nil {
		t.Fatal("expected mkdir error")
	}
	// destination is a directory, so rename fails and temp file is removed.
	dir2 := t.TempDir()
	if err := os.MkdirAll(BasePath(dir2, "PROJ-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SaveBase(dir2, baseIssue("x")); err == nil {
		t.Fatal("expected rename error")
	}
	entries, _ := os.ReadDir(BaseDir(dir2))
	if len(entries) != 1 {
		t.Fatalf("temp file leaked: %v", entries)
	}
}
