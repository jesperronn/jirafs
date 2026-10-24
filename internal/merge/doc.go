// Package merge performs the three-way merge `jirafs pull` applies to an
// issue whose local file has unsynced edits: per field, remote-only changes
// are taken, local-only changes are kept, and fields changed on both sides to
// different values are reported as conflicts. Long text is merged by line.
package merge
