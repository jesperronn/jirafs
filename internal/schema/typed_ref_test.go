package schema

import "testing"

func TestParseTypedRef_valid(t *testing.T) {
	tests := []struct {
		input string
		want  TypedRef
	}{
		{input: "user:jesper", want: TypedRef{Type: RefUser, Value: "jesper"}},
		{input: "project:abc", want: TypedRef{Type: RefProject, Value: "abc"}},
		{input: "status:in-progress", want: TypedRef{Type: RefStatus, Value: "in-progress"}},
		{input: "sprint:platform-2026-w25", want: TypedRef{Type: RefSprint, Value: "platform-2026-w25"}},
		{input: "version:abc-1.4.0", want: TypedRef{Type: RefVersion, Value: "abc-1.4.0"}},
		{input: "epic:ABC-1", want: TypedRef{Type: RefEpic, Value: "ABC-1"}},
		{input: "issue:ABC-123", want: TypedRef{Type: RefIssue, Value: "ABC-123"}},
		{input: "issuetype:story", want: TypedRef{Type: RefIssueType, Value: "story"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypedRef(tt.input)
			if err != nil {
				t.Fatalf("ParseTypedRef(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseTypedRef(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTypedRef_invalid(t *testing.T) {
	tests := []string{
		"user",         // no colon
		"user:",        // empty value
		":jesper",      // empty type
		"unknown:x",    // unknown type
		"",             // empty string
	// "user:val:ue" is valid — only the first colon separates type from value
	}
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			_, err := ParseTypedRef(tc)
			if err == nil {
				t.Errorf("ParseTypedRef(%q) expected error, got nil", tc)
			}
		})
	}
}

func TestTypedRef_String(t *testing.T) {
	r := TypedRef{Type: RefIssue, Value: "ABC-123"}
	got := r.String()
	want := "issue:ABC-123"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestTypedRef_String_roundtrip(t *testing.T) {
	original := "sprint:platform-2026-w25"
	parsed, err := ParseTypedRef(original)
	if err != nil {
		t.Fatalf("ParseTypedRef(%q) unexpected error: %v", original, err)
	}
	serialized := parsed.String()
	if serialized != original {
		t.Errorf("roundtrip: got %q, want %q", serialized, original)
	}
}

func TestTypedRef_IsZero(t *testing.T) {
	var z TypedRef
	if !z.IsZero() {
		t.Error("zero value should be IsZero")
	}
	r := TypedRef{Type: RefUser, Value: "jesper"}
	if r.IsZero() {
		t.Error("non-zero value should not be IsZero")
	}
}

func TestTypedRef_Equals(t *testing.T) {
	r1 := TypedRef{Type: RefIssue, Value: "ABC-123"}
	r2 := TypedRef{Type: RefIssue, Value: "ABC-123"}
	r3 := TypedRef{Type: RefIssue, Value: "ABC-456"}
	r4 := TypedRef{Type: RefSprint, Value: "ABC-123"}

	if !r1.Equals(r2) {
		t.Error("equal refs should be Equals")
	}
	if r1.Equals(r3) {
		t.Error("different value should not be Equals")
	}
	if r1.Equals(r4) {
		t.Error("different type should not be Equals")
	}
}

func TestIsValidRefType(t *testing.T) {
	for _, rt := range ValidRefTypes {
		if !IsValidRefType(rt) {
			t.Errorf("IsValidRefType(%q) should be true", rt)
		}
	}
	if IsValidRefType("bogus") {
		t.Error("IsValidRefType(bogus) should be false")
	}
}
