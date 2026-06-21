package schema

import (
	"testing"
	"time"
)

func TestRemoteMetadata_IsZero(t *testing.T) {
	var zero RemoteMetadata
	if !zero.IsZero() {
		t.Error("zero RemoteMetadata should be IsZero")
	}

	filled := RemoteMetadata{
		RemoteVersion: "42",
		ContentHash:   "abc123",
		SyncTime:      time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC),
	}
	if filled.IsZero() {
		t.Error("non-zero RemoteMetadata should not be IsZero")
	}
}

func TestRemoteMetadata_IsZero_partial(t *testing.T) {
	partial := RemoteMetadata{
		RemoteVersion: "42",
	}
	if partial.IsZero() {
		t.Error("partial RemoteMetadata should not be IsZero")
	}

	partial2 := RemoteMetadata{
		ContentHash: "abc",
	}
	if partial2.IsZero() {
		t.Error("partial RemoteMetadata should not be IsZero")
	}

	partial3 := RemoteMetadata{
		SyncTime: time.Now(),
	}
	if partial3.IsZero() {
		t.Error("partial RemoteMetadata should not be IsZero")
	}
}
