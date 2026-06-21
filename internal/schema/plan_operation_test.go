package schema

import "testing"

func TestOperationTypeConstants(t *testing.T) {
	expected := []OperationType{OpSet, OpAppend, OpRemove}
	for _, op := range expected {
		if op == "" {
			t.Errorf("operation type should not be empty: %q", op)
		}
	}
}

func TestIsValidOperationType(t *testing.T) {
	if !IsValidOperationType(OpSet) {
		t.Error("OpSet should be valid")
	}
	if !IsValidOperationType(OpAppend) {
		t.Error("OpAppend should be valid")
	}
	if !IsValidOperationType(OpRemove) {
		t.Error("OpRemove should be valid")
	}
	if IsValidOperationType("unknown") {
		t.Error("unknown operation type should be invalid")
	}
	if IsValidOperationType("") {
		t.Error("empty operation type should be invalid")
	}
}

func TestPlanOperation_IsZero(t *testing.T) {
	var zero PlanOperation
	if !zero.IsZero() {
		t.Error("zero PlanOperation should be IsZero")
	}

	nonZero := PlanOperation{
		Field: EditableFieldSummary,
		Type:  OpSet,
		Value: "New title",
	}
	if nonZero.IsZero() {
		t.Error("non-zero PlanOperation should not be IsZero")
	}
}

func TestPlanOperation_IsZero_partial(t *testing.T) {
	partial := PlanOperation{Field: EditableFieldSummary, Type: OpSet}
	if partial.IsZero() {
		t.Error("partial PlanOperation should not be IsZero")
	}

	partial2 := PlanOperation{Field: EditableFieldSummary, Value: "x"}
	if partial2.IsZero() {
		t.Error("partial PlanOperation should not be IsZero")
	}

	partial3 := PlanOperation{Type: OpSet, Value: "x"}
	if partial3.IsZero() {
		t.Error("partial PlanOperation should not be IsZero")
	}
}

func TestPlanOperation_String(t *testing.T) {
	op := PlanOperation{
		Field: EditableFieldSummary,
		Type:  OpSet,
		Value: "New title",
	}
	want := "set:summary:New title"
	got := op.String()
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestPlanOperation_String_empty(t *testing.T) {
	op := PlanOperation{}
	want := "::"
	got := op.String()
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestPlanOperation_Equals(t *testing.T) {
	a := PlanOperation{Field: EditableFieldSummary, Type: OpSet, Value: "x"}
	b := PlanOperation{Field: EditableFieldSummary, Type: OpSet, Value: "x"}
	c := PlanOperation{Field: EditableFieldDescription, Type: OpSet, Value: "x"}
	d := PlanOperation{Field: EditableFieldSummary, Type: OpAppend, Value: "x"}
	e := PlanOperation{Field: EditableFieldSummary, Type: OpSet, Value: "y"}

	if !a.Equals(b) {
		t.Error("a and b should be equal")
	}
	if a.Equals(c) {
		t.Error("a and c should not be equal (different field)")
	}
	if a.Equals(d) {
		t.Error("a and d should not be equal (different type)")
	}
	if a.Equals(e) {
		t.Error("a and e should not be equal (different value)")
	}
}

func TestPlanOperation_Equals_zero(t *testing.T) {
	var zero PlanOperation
	zero2 := PlanOperation{}
	if !zero.Equals(zero2) {
		t.Error("two zero PlanOperations should be equal")
	}
}
