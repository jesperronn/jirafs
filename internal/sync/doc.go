// Package sync applies a validated sync plan to bring remote issue state
// in line with local state.
//
// Sync takes a plan built by the plan package, validates it against the
// current remote state, and applies the operations. For a no-op plan
// (zero operations), Sync returns without mutation.
package sync
