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

// B024a: zero-value, partial, and invalid-state edge cases for PlanOperation.

func TestPlanOperation_invalidOperationType(t *testing.T) {
	// A PlanOperation with an unknown OperationType should still be usable.
	op := PlanOperation{
		Field: EditableFieldSummary,
		Type:  "custom_op",
		Value: "some value",
	}
	if op.Type != "custom_op" {
		t.Errorf("expected Type %q, got %q", "custom_op", op.Type)
	}
	if !IsValidOperationType(op.Type) {
		t.Log("IsValidOperationType correctly rejects unknown type")
	}
	// String should still render.
	want := "custom_op:summary:some value"
	if got := op.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	// IsZero should be false since non-zero fields are set.
	if op.IsZero() {
		t.Error("PlanOperation with invalid type but set fields should not be IsZero")
	}
}

func TestPlanOperation_invalidEditableField(t *testing.T) {
	// A PlanOperation with an unknown EditableField should still be usable.
	op := PlanOperation{
		Field: "custom_field",
		Type:  OpSet,
		Value: "some value",
	}
	if got := op.String(); got != "set:custom_field:some value" {
		t.Errorf("String() = %q, want %q", got, "set:custom_field:some value")
	}
	if op.IsZero() {
		t.Error("PlanOperation with invalid field but set Type/Value should not be IsZero")
	}
}

func TestPlanOperation_specialCharactersInValue(t *testing.T) {
	// Values containing special characters (colons, newlines, unicode) should
	// render correctly through String().
	testCases := []struct {
		name  string
		value string
	}{
		{"colon_in_value", "summary:with:colons"},
		{"newline_in_value", "line1\nline2"},
		{"unicode_value", "日本語の概要"},
		{"emoji_value", "fix 🐛 bug"},
		{"empty_value", ""},
		{"whitespace_value", "   "},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			op := PlanOperation{
				Field: EditableFieldSummary,
				Type:  OpSet,
				Value: tc.value,
			}
			want := "set:summary:" + tc.value
			if got := op.String(); got != want {
				t.Errorf("String() = %q, want %q", got, want)
			}
		})
	}
}

func TestPlanOperation_longValue(t *testing.T) {
	// A PlanOperation with a very long Value should still render.
	longValue := "x"
	for i := 0; i < 10000; i++ {
		longValue += "x"
	}
	op := PlanOperation{
		Field: EditableFieldDescription,
		Type:  OpSet,
		Value: longValue,
	}
	want := "set:description:" + longValue
	if got := op.String(); got != want {
		t.Errorf("String() length = %d, want %d", len(got), len(want))
	}
}

func TestPlanOperation_emptyField(t *testing.T) {
	// A PlanOperation with an empty Field but valid Type and Value.
	op := PlanOperation{
		Field: "",
		Type:  OpSet,
		Value: "some value",
	}
	if op.IsZero() {
		t.Error("PlanOperation with empty field but set Type/Value should not be IsZero")
	}
	want := "set::some value"
	if got := op.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestPlanOperation_allFieldsSet(t *testing.T) {
	// A PlanOperation with all three fields set (non-zero).
	op := PlanOperation{
		Field: EditableFieldStatus,
		Type:  OpAppend,
		Value: "new label",
	}
	if op.IsZero() {
		t.Error("PlanOperation with all fields set should not be IsZero")
	}
	want := "append:status:new label"
	if got := op.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	// Equals should work correctly.
	op2 := PlanOperation{
		Field: EditableFieldStatus,
		Type:  OpAppend,
		Value: "new label",
	}
	if !op.Equals(op2) {
		t.Error("two PlanOperations with all fields set should be equal")
	}
}

func TestPlanOperation_appendAndRemoveTypes(t *testing.T) {
	// PlanOperation with OpAppend and OpRemove types.
	appendOp := PlanOperation{
		Field: EditableFieldLabels,
		Type:  OpAppend,
		Value: "new-label",
	}
	if got := appendOp.String(); got != "append:labels:new-label" {
		t.Errorf("append String() = %q, want %q", got, "append:labels:new-label")
	}

	removeOp := PlanOperation{
		Field: EditableFieldLabels,
		Type:  OpRemove,
		Value: "old-label",
	}
	if got := removeOp.String(); got != "remove:labels:old-label" {
		t.Errorf("remove String() = %q, want %q", got, "remove:labels:old-label")
	}

	if !appendOp.Equals(appendOp) {
		t.Error("appendOp should equal itself")
	}
	if appendOp.Equals(removeOp) {
		t.Error("appendOp and removeOp should not be equal")
	}
}

func TestPlanOperation_IsZero_edgeCases(t *testing.T) {
	// IsZero with only Field set.
	op1 := PlanOperation{Field: EditableFieldSummary}
	if op1.IsZero() {
		t.Error("PlanOperation with only Field set should not be IsZero")
	}

	// IsZero with only Type set.
	op2 := PlanOperation{Type: OpSet}
	if op2.IsZero() {
		t.Error("PlanOperation with only Type set should not be IsZero")
	}

	// IsZero with only Value set.
	op3 := PlanOperation{Value: "some value"}
	if op3.IsZero() {
		t.Error("PlanOperation with only Value set should not be IsZero")
	}
}

func TestPlanOperation_Equals_edgeCases(t *testing.T) {
	// Equals with different Field values.
	op1 := PlanOperation{Field: EditableFieldSummary, Type: OpSet, Value: "x"}
	op2 := PlanOperation{Field: EditableFieldDescription, Type: OpSet, Value: "x"}
	if op1.Equals(op2) {
		t.Error("PlanOperations with different Field should not be equal")
	}

	// Equals with different Type values.
	op3 := PlanOperation{Field: EditableFieldSummary, Type: OpSet, Value: "x"}
	op4 := PlanOperation{Field: EditableFieldSummary, Type: OpAppend, Value: "x"}
	if op3.Equals(op4) {
		t.Error("PlanOperations with different Type should not be equal")
	}

	// Equals with different Value strings.
	op5 := PlanOperation{Field: EditableFieldSummary, Type: OpSet, Value: "a"}
	op6 := PlanOperation{Field: EditableFieldSummary, Type: OpSet, Value: "b"}
	if op5.Equals(op6) {
		t.Error("PlanOperations with different Value should not be equal")
	}

	// Equals between zero and non-zero PlanOperation.
	var zero PlanOperation
	nonZero := PlanOperation{Field: EditableFieldSummary, Type: OpSet, Value: "x"}
	if zero.Equals(nonZero) {
		t.Error("zero PlanOperation should not equal non-zero PlanOperation")
	}
	if nonZero.Equals(zero) {
		t.Error("non-zero PlanOperation should not equal zero PlanOperation")
	}
}
