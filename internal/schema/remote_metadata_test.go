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

func TestRemoteMetadata_IsZero_pinned(t *testing.T) {
	// A RemoteMetadata with only Pinned set is not zero (Pinned is set).
	pinned := RemoteMetadata{
		Pinned: true,
	}
	if pinned.IsZero() {
		t.Error("RemoteMetadata with only Pinned set should not be IsZero")
	}
	// But with other fields set, it should not be zero.
	filled := RemoteMetadata{
		RemoteVersion: "1",
		Pinned:        true,
	}
	if filled.IsZero() {
		t.Error("RemoteMetadata with RemoteVersion and Pinned should not be IsZero")
	}
}

func TestRemoteMetadata_State_draft(t *testing.T) {
	r := RemoteMetadata{StateFile: "draft"}
	if got := r.State(); got != StateDraft {
		t.Errorf("State() = %q, want %q", got, StateDraft)
	}
}

func TestRemoteMetadata_State_archived(t *testing.T) {
	r := RemoteMetadata{StateFile: "archived"}
	if got := r.State(); got != StateArchived {
		t.Errorf("State() = %q, want %q", got, StateArchived)
	}
}

func TestRemoteMetadata_State_unsynced(t *testing.T) {
	r := RemoteMetadata{}
	if got := r.State(); got != StateUnsynced {
		t.Errorf("State() = %q, want %q", got, StateUnsynced)
	}
}

func TestRemoteMetadata_State_synced(t *testing.T) {
	r := RemoteMetadata{
		RemoteVersion: "1",
		ContentHash:   "abc",
		SyncTime:      time.Now(),
	}
	if got := r.State(); got != StateSynced {
		t.Errorf("State() = %q, want %q", got, StateSynced)
	}
}

func TestRemoteMetadata_State_synced_with_state_file(t *testing.T) {
	// When StateFile is empty but remote metadata is present,
	// the state should be synced (not draft or archived).
	r := RemoteMetadata{
		RemoteVersion: "1",
		ContentHash:   "abc",
		SyncTime:      time.Now(),
	}
	if got := r.State(); got != StateSynced {
		t.Errorf("State() = %q, want %q", got, StateSynced)
	}
}

func TestRemoteMetadata_State_draft_with_remote_metadata(t *testing.T) {
	// StateFile takes priority: a draft can have partial remote metadata
	// (e.g. after upload but before full sync metadata is written).
	r := RemoteMetadata{
		StateFile:     "draft",
		RemoteVersion: "1",
	}
	if got := r.State(); got != StateDraft {
		t.Errorf("State() = %q, want %q", got, StateDraft)
	}
}

func TestRemoteMetadata_State_archived_with_remote_metadata(t *testing.T) {
	// StateFile takes priority for archived issues.
	r := RemoteMetadata{
		StateFile:     "archived",
		ContentHash:   "abc",
	}
	if got := r.State(); got != StateArchived {
		t.Errorf("State() = %q, want %q", got, StateArchived)
	}
}

func TestRemoteMetadata_IsSyncable_synced(t *testing.T) {
	r := RemoteMetadata{
		RemoteVersion: "1",
		ContentHash:   "abc",
	}
	if !r.IsSyncable() {
		t.Error("synced RemoteMetadata should be IsSyncable")
	}
}

func TestRemoteMetadata_IsSyncable_unsynced(t *testing.T) {
	r := RemoteMetadata{}
	if r.IsSyncable() {
		t.Error("unsynced RemoteMetadata should not be IsSyncable")
	}
}

func TestRemoteMetadata_IsSyncable_draft(t *testing.T) {
	r := RemoteMetadata{StateFile: "draft"}
	if r.IsSyncable() {
		t.Error("draft RemoteMetadata should not be IsSyncable")
	}
}

func TestRemoteMetadata_IsSyncable_archived(t *testing.T) {
	r := RemoteMetadata{StateFile: "archived"}
	if r.IsSyncable() {
		t.Error("archived RemoteMetadata should not be IsSyncable")
	}
}

func TestIssueState_constants(t *testing.T) {
	// Verify the constants have the expected string values.
	if StateUnsynced != "unsynced" {
		t.Errorf("StateUnsynced = %q, want %q", StateUnsynced, "unsynced")
	}
	if StateSynced != "synced" {
		t.Errorf("StateSynced = %q, want %q", StateSynced, "synced")
	}
	if StateDraft != "draft" {
		t.Errorf("StateDraft = %q, want %q", StateDraft, "draft")
	}
	if StateArchived != "archived" {
		t.Errorf("StateArchived = %q, want %q", StateArchived, "archived")
	}
}

func TestRemoteMetadata_State_empty_state_file_zero_metadata(t *testing.T) {
	// When StateFile is empty and metadata is zero, should be unsynced.
	r := RemoteMetadata{StateFile: ""}
	if got := r.State(); got != StateUnsynced {
		t.Errorf("State() = %q, want %q", got, StateUnsynced)
	}
}

func TestRemoteMetadata_State_unknown_state_file(t *testing.T) {
	// Unknown StateFile values fall through to IsZero check.
	r := RemoteMetadata{
		StateFile: "unknown",
	}
	if got := r.State(); got != StateUnsynced {
		t.Errorf("State() = %q, want %q", got, StateUnsynced)
	}
}

func TestRemoteMetadata_State_unknown_state_file_with_metadata(t *testing.T) {
	// Unknown StateFile with metadata present should be synced.
	r := RemoteMetadata{
		StateFile:     "unknown",
		RemoteVersion: "1",
	}
	if got := r.State(); got != StateSynced {
		t.Errorf("State() = %q, want %q", got, StateSynced)
	}
}
