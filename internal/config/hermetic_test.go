package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveFlagNeverRunsRealCredentialCLI(t *testing.T) {
	home := t.TempDir()
	if os.Getenv("HOME") == "" || strings.HasPrefix(os.Getenv("HOME"), "/Users/") {
		t.Fatalf("HOME not isolated: %q", os.Getenv("HOME"))
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "invoked")
	stub := "#!/bin/sh\necho invoked >> " + log + "\nexit 1\n"
	for _, n := range []string{"op", "security"} {
		if err := os.WriteFile(filepath.Join(bin, n), []byte(stub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", home)
	t.Setenv("JIRAFS_ALLOW_LIVE_ENDPOINTS", "1")
	opCredentialCache.Delete("hermetic-item")

	_, err := ResolveOPCredential(CredentialRef{Scheme: "op", Target: "hermetic-item"})
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected blocked error, got %v", err)
	}
	opCredentialCache.Delete("hermetic-item")
	if _, statErr := os.Stat(log); statErr == nil {
		t.Fatal("external credential command was invoked")
	}
}
