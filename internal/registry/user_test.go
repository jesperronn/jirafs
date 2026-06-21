package registry

import "testing"

func TestUser_IsZero(t *testing.T) {
	var zero User
	if !zero.IsZero() {
		t.Error("zero User should be IsZero")
	}

	filled := User{
		AccountID:   "712020:abcd",
		DisplayName: "Jesper Ronn",
		Email:       "jesper@example.com",
		Active:      true,
	}
	if filled.IsZero() {
		t.Error("non-zero User should not be IsZero")
	}
}

func TestUser_IsZero_partial(t *testing.T) {
	partial := User{AccountID: "712020:abcd"}
	if partial.IsZero() {
		t.Error("partial User should not be IsZero")
	}

	partial2 := User{DisplayName: "Jesper"}
	if partial2.IsZero() {
		t.Error("partial User should not be IsZero")
	}

	partial3 := User{Email: "jesper@example.com"}
	if partial3.IsZero() {
		t.Error("partial User should not be IsZero")
	}
}
