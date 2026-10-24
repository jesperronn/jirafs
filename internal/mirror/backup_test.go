package mirror

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveBackup(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 1, 2, 10, 30, 0, 0, time.UTC)
	p1, err := SaveBackup(dir, "PLAT-1", "first", now)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(BackupDir(dir), "PLAT-1-20260102T103000Z.md"); p1 != want {
		t.Errorf("path = %q, want %q", p1, want)
	}
	p2, err := SaveBackup(dir, "PLAT-1", "second", now)
	if err != nil {
		t.Fatal(err)
	}
	if p2 == p1 || filepath.Base(p2) != "PLAT-1-20260102T103000Z-2.md" {
		t.Errorf("second path = %q", p2)
	}
	p3, _ := SaveBackup(dir, "PLAT-1", "third", now)
	if filepath.Base(p3) != "PLAT-1-20260102T103000Z-3.md" {
		t.Errorf("third path = %q", p3)
	}
	for p, want := range map[string]string{p1: "first", p2: "second", p3: "third"} {
		if got, _ := os.ReadFile(p); string(got) != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}
}

func TestSaveBackupErrors(t *testing.T) {
	now := time.Now()
	if _, err := SaveBackup("", "PLAT-1", "x", now); err == nil {
		t.Error("empty mirror dir: want error")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".jirafs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(BackupDir(dir), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveBackup(dir, "PLAT-1", "x", now); err == nil {
		t.Error("backup dir is a file: want error")
	}
	ro := t.TempDir()
	if err := os.MkdirAll(BackupDir(ro), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(BackupDir(ro), 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(BackupDir(ro), 0o755) }()
	if os.Geteuid() != 0 {
		if _, err := SaveBackup(ro, "PLAT-1", "x", now); err == nil {
			t.Error("read-only backup dir: want error")
		}
	}
}
