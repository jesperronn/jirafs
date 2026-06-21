package archive

import "testing"

func TestServiceFunc_Archive(t *testing.T) {
	called := false
	f := ServiceFunc(func(eligible string, mirrorDir, localDir, issuePath string) error {
		called = true
		if eligible != "PROJ-123" {
			t.Errorf("eligible = %q, want %q", eligible, "PROJ-123")
		}
		if mirrorDir != "/mirror" {
			t.Errorf("mirrorDir = %q, want %q", mirrorDir, "/mirror")
		}
		if localDir != "/local" {
			t.Errorf("localDir = %q, want %q", localDir, "/local")
		}
		if issuePath != "/local/PROJ-123.md" {
			t.Errorf("issuePath = %q, want %q", issuePath, "/local/PROJ-123.md")
		}
		return nil
	})

	if err := f.Archive("PROJ-123", "/mirror", "/local", "/local/PROJ-123.md"); err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if !called {
		t.Error("ServiceFunc was not called")
	}
}
