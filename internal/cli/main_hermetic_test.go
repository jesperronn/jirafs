package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jirafs/jirafs/internal/testutil"
)

// TestMain runs the package hermetically and fails the package if any test
// leaves a new file in the package directory (the cwd of `go test`).
func TestMain(m *testing.M) {
	before := snapshotFiles(".")
	code := testutil.RunHermeticCode(m)
	if code == 0 {
		if stray := newFilesSince(before, snapshotFiles(".")); len(stray) > 0 {
			fmt.Fprintf(os.Stderr, "cli tests left files in the package directory (use t.TempDir()): %v\n", stray)
			code = 1
		}
	}
	os.Exit(code)
}

// snapshotFiles returns the set of file paths under dir.
func snapshotFiles(dir string) map[string]bool {
	files := map[string]bool{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files[p] = true
		}
		return nil
	})
	return files
}

// newFilesSince returns the sorted paths present in after but not in before.
func newFilesSince(before, after map[string]bool) []string {
	var out []string
	for p := range after {
		if !before[p] {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func TestNewFilesSince(t *testing.T) {
	before := map[string]bool{"a.go": true}
	after := map[string]bool{"a.go": true, "z.md": true, "b.md": true}
	got := newFilesSince(before, after)
	if len(got) != 2 || got[0] != "b.md" || got[1] != "z.md" {
		t.Fatalf("newFilesSince = %v, want [b.md z.md]", got)
	}
	if got := newFilesSince(after, after); len(got) != 0 {
		t.Fatalf("newFilesSince(same) = %v, want empty", got)
	}
}

func TestSnapshotFilesSeesNewFile(t *testing.T) {
	dir := t.TempDir()
	before := snapshotFiles(dir)
	p := filepath.Join(dir, "stray.md")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := newFilesSince(before, snapshotFiles(dir))
	if len(got) != 1 || got[0] != p {
		t.Fatalf("newFilesSince = %v, want [%s]", got, p)
	}
}
