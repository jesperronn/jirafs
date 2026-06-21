package registry

import "testing"

func TestStatus_IsZero(t *testing.T) {
	var zero Status
	if !zero.IsZero() {
		t.Error("zero Status should be IsZero")
	}

	filled := Status{
		Name:        "In Progress",
		Category:    "InProgress",
		Description: "Work is being done on this issue.",
	}
	if filled.IsZero() {
		t.Error("non-zero Status should not be IsZero")
	}
}

func TestStatus_IsZero_partial(t *testing.T) {
	partial := Status{Name: "Done"}
	if partial.IsZero() {
		t.Error("partial Status should not be IsZero")
	}

	partial2 := Status{Category: "Done"}
	if partial2.IsZero() {
		t.Error("partial Status should not be IsZero")
	}

	partial3 := Status{Description: "Completed"}
	if partial3.IsZero() {
		t.Error("partial Status should not be IsZero")
	}
}
