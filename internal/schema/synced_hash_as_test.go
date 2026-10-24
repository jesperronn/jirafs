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

func TestFrontmatterText(t *testing.T) {
	got, ok := FrontmatterText("---\nkey: A-1\nx: 2\n---\nbody\n")
	if !ok || got != "key: A-1\nx: 2\n" {
		t.Errorf("got %q, %v", got, ok)
	}
	if _, ok := FrontmatterText("plain"); ok {
		t.Error("no frontmatter must report false")
	}
}

func TestRemoveFrontmatterKey(t *testing.T) {
	doc := "---\nkey: A-1\nconflicts:\n    - field: summary\n      remote: x\n    - field: labels\n      remote: y\nafter: 1\n---\nbody\nconflicts: keep\n"
	want := "---\nkey: A-1\nafter: 1\n---\nbody\nconflicts: keep\n"
	if got := RemoveFrontmatterKey(doc, "conflicts"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := RemoveFrontmatterKey("---\nkey: A-1\n---\n", "conflicts"); got != "---\nkey: A-1\n---\n" {
		t.Errorf("absent key changed the doc: %q", got)
	}
	if got := RemoveFrontmatterKey("plain", "conflicts"); got != "plain" {
		t.Errorf("no frontmatter: %q", got)
	}
}
