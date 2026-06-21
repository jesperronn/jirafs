package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewGolden(t *testing.T) {
	dir := t.TempDir()

	g, err := NewGolden(dir)
	if err != nil {
		t.Fatalf("NewGolden: %v", err)
	}
	if g == nil {
		t.Fatal("NewGolden returned nil")
	}
	if g.root != dir {
		t.Errorf("root = %q; want %q", g.root, dir)
	}

	_, err = NewGolden(filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Fatal("NewGolden with nonexistent dir: expected error")
	}
}

func TestMustGolden(t *testing.T) {
	dir := t.TempDir()
	g := MustGolden(dir)
	if g == nil {
		t.Fatal("MustGolden returned nil")
	}
}

func TestMustGoldenPanicsOnBadRoot(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustGolden() did not panic for bad root")
		}
	}()
	MustGolden("/nonexistent/path/that/does/not/exist")
}

func TestNewGoldenRelativePath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	relative := filepath.Join("goldens", "nested")
	if err := os.MkdirAll(filepath.Join(root, relative), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	g, err := NewGolden(relative)
	if err != nil {
		t.Fatalf("NewGolden() error = %v", err)
	}
	if !filepath.IsAbs(g.root) {
		t.Fatalf("root = %q, want absolute path", g.root)
	}
}

func TestGoldenPathUnique(t *testing.T) {
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

func TestGoldenExistsUnique(t *testing.T) {
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

func TestWriteGoldenUnique(t *testing.T) {
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

func TestAssertEqualGoldenUpdateCreates(t *testing.T) {
	// AssertEqual writes to CWD/t.Name()/key.
	AssertEqual(t, "new/key", "golden output", true)

	// Verify the golden file was created at CWD/t.Name()/new/key
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	goldenPath := filepath.Join(cwd, t.Name(), "new/key")
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file not created: %v", err)
	}
	if string(data) != "golden output" {
		t.Errorf("golden content = %q; want %q", string(data), "golden output")
	}

	// Clean up the golden file created by updateGolden
	goldenDir := filepath.Join(cwd, t.Name(), "new")
	_ = os.Remove(goldenPath)
	_ = os.Remove(goldenDir)
	_ = os.Remove(filepath.Dir(goldenDir))
}

func TestAssertEqualGoldenMatch(t *testing.T) {
	// Create golden file in the path AssertEqual will look for: CWD/t.Name()/key
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	goldenDir := filepath.Join(cwd, t.Name(), "match")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goldenDir, "ok.txt"), []byte("exact match"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not fail when content matches
	AssertEqual(t, "match/ok.txt", "exact match", false)
}

func TestAssertRaw(t *testing.T) {
	// Create golden file in the path AssertEqual (called by AssertRaw) will look for
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	goldenDir := filepath.Join(cwd, t.Name(), "raw")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goldenDir, "data.bin"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}

	AssertRaw(t, "raw/data.bin", []byte{0x00, 0x01, 0x02}, false)
}

func TestMustReadGolden(t *testing.T) {
	dir := t.TempDir()
	goldenDir := filepath.Join(dir, "golden", "read")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goldenDir, "file.txt"), []byte("readable content"), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := NewGolden(dir)
	if err != nil {
		t.Fatalf("NewGolden: %v", err)
	}

	content := MustReadGolden(t, g, "read/file.txt")
	if content != "readable content" {
		t.Errorf("content = %q; want %q", content, "readable content")
	}
}

func TestDiffLinesNoDiff(t *testing.T) {
	result := diffLines("line1\nline2\nline3\n", "line1\nline2\nline3\n")
	if result != "(no line differences)" {
		t.Errorf("diffLines identical: got %q", result)
	}
}

func TestDiffLinesWithDiff(t *testing.T) {
	result := diffLines("line1\nline2\nline3\n", "line1\nCHANGED\nline3\n")
	if result == "(no line differences)" {
		t.Fatal("diffLines should show differences")
	}
	if !strings.Contains(result, "- line2") || !strings.Contains(result, "+ CHANGED") {
		t.Errorf("diffLines missing expected lines: got %q", result)
	}
}

func TestDiffLinesExtraActualLines(t *testing.T) {
	result := diffLines("line1\n", "line1\nextra\n")
	if result == "(no line differences)" {
		t.Fatal("diffLines should show extra lines")
	}
	if !strings.Contains(result, "+ extra") {
		t.Errorf("diffLines missing extra line: got %q", result)
	}
}

func TestDiffLinesExtraExpectedLines(t *testing.T) {
	result := diffLines("line1\nexpected\n", "line1\n")
	if result == "(no line differences)" {
		t.Fatal("diffLines should show missing lines")
	}
	if !strings.Contains(result, "- expected") {
		t.Errorf("diffLines missing expected line: got %q", result)
	}
}
