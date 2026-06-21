package registry

import (
	"testing"
	"time"
)

func TestSprint_IsZero(t *testing.T) {
	var zero Sprint
	if !zero.IsZero() {
		t.Error("zero Sprint should be IsZero")
	}

	now := time.Now()
	filled := Sprint{
		ID:           1001,
		Name:         "Sprint 42",
		State:        "active",
		StartDate:    &now,
		EndDate:      &now,
		CompleteDate: &now,
	}
	if filled.IsZero() {
		t.Error("non-zero Sprint should not be IsZero")
	}
}

func TestSprint_IsZero_partial(t *testing.T) {
	partial := Sprint{ID: 1001}
	if partial.IsZero() {
		t.Error("partial Sprint should not be IsZero")
	}

	partial2 := Sprint{Name: "Sprint 42"}
	if partial2.IsZero() {
		t.Error("partial Sprint should not be IsZero")
	}

	partial3 := Sprint{State: "active"}
	if partial3.IsZero() {
		t.Error("partial Sprint should not be IsZero")
	}
}
