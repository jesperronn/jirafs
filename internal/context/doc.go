// Package context resolves the active project from multiple sources.
//
// Precedence (highest to lowest):
//
//  1. Explicit --project flag
//  2. Cwd mapping (most-specific mirror_dir match)
//  3. Remembered current project from settings state
package context
