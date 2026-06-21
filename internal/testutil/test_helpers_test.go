package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTestFixtures(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	fixturesDir := filepath.Join(root, "fixtures", "sample")
	if err := os.MkdirAll(fixturesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixturesDir, "data.txt"), []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	fixtures := TestFixtures(t)
	got, err := fixtures.Read("sample/data.txt")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got != "fixture" {
		t.Fatalf("Read() = %q, want %q", got, "fixture")
	}
}

func TestTestGolden(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	goldenDir := filepath.Join(root, "golden", "golden", "sample")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goldenDir, "expected.txt"), []byte("golden"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	golden := TestGolden(t)
	got := MustReadGolden(t, golden, "sample/expected.txt")
	if got != "golden" {
		t.Fatalf("MustReadGolden() = %q, want %q", got, "golden")
	}
}
