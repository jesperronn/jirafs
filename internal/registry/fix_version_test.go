package registry

import "testing"

func TestFixVersion_IsZero(t *testing.T) {
	var zero FixVersion
	if !zero.IsZero() {
		t.Error("zero FixVersion should be IsZero")
	}

	filled := FixVersion{
		Name:        "1.4.0",
		Description: "Platform release with bug fixes.",
		Archived:    false,
		Released:    true,
	}
	if filled.IsZero() {
		t.Error("non-zero FixVersion should not be IsZero")
	}
}

func TestFixVersion_IsZero_partial(t *testing.T) {
	partial := FixVersion{Name: "1.4.0"}
	if partial.IsZero() {
		t.Error("partial FixVersion should not be IsZero")
	}

	partial2 := FixVersion{Archived: true}
	if partial2.IsZero() {
		t.Error("partial FixVersion should not be IsZero")
	}

	partial3 := FixVersion{Released: true}
	if partial3.IsZero() {
		t.Error("partial FixVersion should not be IsZero")
	}
}
