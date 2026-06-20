package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFixtures(t *testing.T) {
	dir := t.TempDir()

	// Valid root
	f, err := NewFixtures(dir)
	if err != nil {
		t.Fatalf("NewFixtures: %v", err)
	}
	if f == nil {
		t.Fatal("NewFixtures returned nil")
	}
	if f.root != dir {
		t.Errorf("root = %q; want %q", f.root, dir)
	}

	// Invalid root
	_, err = NewFixtures(filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Fatal("NewFixtures with nonexistent dir: expected error")
	}
}

func TestFixturesRead(t *testing.T) {
	dir := t.TempDir()
	fixtureDir := filepath.Join(dir, "test")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(fixtureDir, "data.txt")
	if err := os.WriteFile(testFile, []byte("hello fixture"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := NewFixtures(dir)
	if err != nil {
		t.Fatalf("NewFixtures: %v", err)
	}

	content, err := f.Read("test/data.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if content != "hello fixture" {
		t.Errorf("content = %q; want %q", content, "hello fixture")
	}
}

func TestFixturesReadMissing(t *testing.T) {
	dir := t.TempDir()
	f, err := NewFixtures(dir)
	if err != nil {
		t.Fatalf("NewFixtures: %v", err)
	}

	_, err = f.Read("nonexistent/file.txt")
	if err == nil {
		t.Fatal("Read missing fixture: expected error")
	}
}

func TestFixturesExists(t *testing.T) {
	dir := t.TempDir()
	fixtureDir := filepath.Join(dir, "test")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(fixtureDir, "data.txt")
	if err := os.WriteFile(testFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := NewFixtures(dir)
	if err != nil {
		t.Fatalf("NewFixtures: %v", err)
	}

	if !f.Exists("test/data.txt") {
		t.Error("Exists: expected true for existing fixture")
	}
	if f.Exists("test/missing.txt") {
		t.Error("Exists: expected false for missing fixture")
	}
}

func TestFixturesWrite(t *testing.T) {
	dir := t.TempDir()
	f, err := NewFixtures(dir)
	if err != nil {
		t.Fatalf("NewFixtures: %v", err)
	}

	err = f.WriteFixture("new/fixture.txt", "new content")
	if err != nil {
		t.Fatalf("WriteFixture: %v", err)
	}

	if !f.Exists("new/fixture.txt") {
		t.Fatal("WriteFixture: fixture not found after write")
	}

	content, err := f.Read("new/fixture.txt")
	if err != nil {
		t.Fatalf("Read after write: %v", err)
	}
	if content != "new content" {
		t.Errorf("content = %q; want %q", content, "new content")
	}
}

func TestFixturesList(t *testing.T) {
	dir := t.TempDir()
	fixtureDir := filepath.Join(dir, "issue", "synced")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixtureDir, "ABC-1.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixtureDir, "ABC-2.md"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := NewFixtures(dir)
	if err != nil {
		t.Fatalf("NewFixtures: %v", err)
	}

	keys, err := f.List("issue.synced")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("List: got %d keys; want 2", len(keys))
	}
}

func TestFixturesListPrefix(t *testing.T) {
	dir := t.TempDir()
	issueDir := filepath.Join(dir, "issue", "synced")
	registryDir := filepath.Join(dir, "registry", "users")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issueDir, "ABC-1.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(registryDir, "users.yaml"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := NewFixtures(dir)
	if err != nil {
		t.Fatalf("NewFixtures: %v", err)
	}

	keys, err := f.List("issue")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("List prefix: got %d keys; want 1", len(keys))
	}
	if keys[0] != "issue.synced/ABC-1.md" {
		t.Errorf("key = %q; want %q", keys[0], "issue.synced/ABC-1.md")
	}
}

func TestFixturesReadRaw(t *testing.T) {
	dir := t.TempDir()
	fixtureDir := filepath.Join(dir, "test")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(fixtureDir, "binary.bin")
	data := []byte{0x00, 0x01, 0x02, 0xFF}
	if err := os.WriteFile(testFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := NewFixtures(dir)
	if err != nil {
		t.Fatalf("NewFixtures: %v", err)
	}

	raw, err := f.ReadRaw("test/binary.bin")
	if err != nil {
		t.Fatalf("ReadRaw: %v", err)
	}
	if len(raw) != 4 {
		t.Fatalf("ReadRaw: got %d bytes; want 4", len(raw))
	}
	for i, b := range data {
		if raw[i] != b {
			t.Errorf("byte[%d] = %02x; want %02x", i, raw[i], b)
		}
	}
}

func TestMustFixtures(t *testing.T) {
	dir := t.TempDir()
	f := MustFixtures(dir)
	if f == nil {
		t.Fatal("MustFixtures returned nil")
	}
}

func TestMustFixturesPanicsOnBadRoot(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustFixtures did not panic on nonexistent root")
		}
	}()
	MustFixtures("/nonexistent/path/that/does/not/exist")
}

func TestGoldenPath(t *testing.T) {
	dir := t.TempDir()
	g, err := NewGolden(dir)
	if err != nil {
		t.Fatalf("NewGolden: %v", err)
	}

	want := filepath.Join(dir, "golden", "issue", "synced", "ABC-1.md")
	got := g.GoldenPath("issue.synced/ABC-1.md")
	if got != want {
		t.Errorf("GoldenPath = %q; want %q", got, want)
	}
}

func TestGoldenExists(t *testing.T) {
	dir := t.TempDir()
	goldenDir := filepath.Join(dir, "golden", "test")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goldenDir, "out.txt"), []byte("expected"), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := NewGolden(dir)
	if err != nil {
		t.Fatalf("NewGolden: %v", err)
	}

	if !g.GoldenExists("test/out.txt") {
		t.Error("GoldenExists: expected true for existing golden")
	}
	if g.GoldenExists("test/missing.txt") {
		t.Error("GoldenExists: expected false for missing golden")
	}
}

func TestWriteGolden(t *testing.T) {
	dir := t.TempDir()
	g, err := NewGolden(dir)
	if err != nil {
		t.Fatalf("NewGolden: %v", err)
	}

	err = WriteGolden(g, "new/file.txt", "golden content")
	if err != nil {
		t.Fatalf("WriteGolden: %v", err)
	}

	if !g.GoldenExists("new/file.txt") {
		t.Fatal("WriteGolden: golden not found after write")
	}

	data, err := os.ReadFile(g.GoldenPath("new/file.txt"))
	if err != nil {
		t.Fatalf("read golden after write: %v", err)
	}
	if string(data) != "golden content" {
		t.Errorf("golden content = %q; want %q", string(data), "golden content")
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if got := truncate([]byte(short), 10); got != short {
		t.Errorf("truncate(short, 10) = %q; want %q", got, short)
	}

	long := "this is a very long string that should be truncated"
	expected := "this is a very long string that should be truncate..."
	if got := truncate([]byte(long), 50); got != expected {
		t.Errorf("truncate(long, 50) = %q; want %q", got, expected)
	}
}
