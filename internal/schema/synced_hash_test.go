package schema

import (
	"strings"
	"testing"
)

const hashDoc = "---\nkey: 'P-1'\nsync_time: '2026-01-01T00:00:00Z'\nsummary: 'Hi'\n---\n## Description\nbody\n\n"

func TestStampSyncedHash_RoundTrip(t *testing.T) {
	stamped := StampSyncedHash(hashDoc)
	if got := SyncedHash(stamped); got != ContentHash(hashDoc) {
		t.Fatalf("SyncedHash = %q, want %q", got, ContentHash(hashDoc))
	}
	if StampSyncedHash(stamped) != stamped {
		t.Fatal("stamping is not idempotent")
	}
	if _, pe := ParseIssue(stamped); pe != nil {
		t.Fatalf("stamped doc no longer parses: %v", pe)
	}
}

func TestContentHash_IgnoresVolatileFields(t *testing.T) {
	changed := strings.Replace(hashDoc, "2026-01-01", "2027-02-02", 1)
	if ContentHash(hashDoc) != ContentHash(changed) {
		t.Fatal("sync_time must not affect the hash")
	}
	if ContentHash(hashDoc) != ContentHash(StampSyncedHash(hashDoc)) {
		t.Fatal("synced_hash must not affect the hash")
	}
	edited := strings.Replace(hashDoc, "body", "edited", 1)
	if ContentHash(hashDoc) == ContentHash(edited) {
		t.Fatal("body edit must change the hash")
	}
}

func TestSyncedHash_NoFrontmatter(t *testing.T) {
	if SyncedHash("plain") != "" || StampSyncedHash("plain") != "plain" {
		t.Fatal("documents without frontmatter must be left alone")
	}
}
