package cli

import (
	"fmt"
	"io"
	"os"
)

// dryRunConflicts reports (and prints an error for) the invalid combination
// of --dry-run with --apply. cmd is the command prefix, e.g. "jirafs push".
func dryRunConflicts(stderr io.Writer, cmd string, dry, apply bool) bool {
	if dry && apply {
		fmt.Fprintf(stderr, "%s: --dry-run and --apply cannot be used together; drop one of them\n", cmd)
		return true
	}
	return false
}

// dryRunDone prints the closing line of every dry run: nothing was changed
// and the exact command to run for real.
func dryRunDone(w io.Writer, realCmd string) {
	fmt.Fprintf(w, "dry run: nothing was written; to run it for real: %s\n", realCmd)
}

// writeVerb returns "overwrite" when path exists, otherwise "create".
func writeVerb(path string) string {
	if _, err := os.Stat(path); err == nil {
		return "overwrite"
	}
	return "create"
}
