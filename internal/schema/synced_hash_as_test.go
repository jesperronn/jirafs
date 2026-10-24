package schema

import "testing"

func TestStampSyncedHashAs(t *testing.T) {
	doc := "---\nkey: A-1\nsynced_hash: old\nsummary: s\n---\nbody\n"
	got := StampSyncedHashAs(doc, "abc")
	if SyncedHash(got) != "abc" {
		t.Errorf("hash = %q in\n%s", SyncedHash(got), got)
	}
	if ContentHash(got) != ContentHash(doc) {
		t.Error("stamping changed the content hash")
	}
	if got := StampSyncedHashAs("plain", "abc"); got != "plain" {
		t.Errorf("no frontmatter: %q", got)
	}
}

func TestAppendFrontmatter(t *testing.T) {
	got := AppendFrontmatter("---\nkey: A-1\n---\nbody\n", "extra: 1\n")
	if want := "---\nkey: A-1\nextra: 1\n---\nbody\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := AppendFrontmatter("plain", "x: 1\n"); got != "plain" {
		t.Errorf("no frontmatter: %q", got)
	}
}
