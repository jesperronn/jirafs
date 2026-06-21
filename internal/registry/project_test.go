package registry

import "testing"

func TestProject_IsZero(t *testing.T) {
	var zero Project
	if !zero.IsZero() {
		t.Error("zero Project should be IsZero")
	}

	filled := Project{
		Key:         "ABC",
		Name:        "A Big Project",
		ID:          "10000",
		Avatar:      "https://example.com/avatar.png",
		Lead:        "712020:abcd",
		ProjectType: "software",
	}
	if filled.IsZero() {
		t.Error("non-zero Project should not be IsZero")
	}
}

func TestProject_IsZero_partial(t *testing.T) {
	partial := Project{Key: "ABC"}
	if partial.IsZero() {
		t.Error("partial Project should not be IsZero")
	}

	partial2 := Project{Name: "A Big Project"}
	if partial2.IsZero() {
		t.Error("partial Project should not be IsZero")
	}

	partial3 := Project{ID: "10000"}
	if partial3.IsZero() {
		t.Error("partial Project should not be IsZero")
	}
}
