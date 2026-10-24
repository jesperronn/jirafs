package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// commitPush commits what a `push --apply` wrote to Jira and the local files
// that went with it. Only successful results with a remote write are
// committed; failed and skipped targets are listed in the body. A pushed
// file is committed in full even when it had uncommitted edits, because those
// edits were just pushed. mirror.yml is included unless the user had edited
// it beforehand. It never fails the push.
func (g *gitTracker) commitPush(results []pushResult, host string, now time.Time, stdout, stderr io.Writer) {
	if g == nil {
		return
	}
	var pushed, notPushed []pushResult
	var failedKeys []string
	attempted := 0
	for _, r := range results {
		switch {
		case r.status == pushOK && r.summary != "":
			pushed = append(pushed, r)
			attempted++
		case r.status == pushFailed:
			notPushed = append(notPushed, r)
			failedKeys = append(failedKeys, r.name)
			attempted++
		case r.status == pushSkipped:
			notPushed = append(notPushed, r)
		}
	}
	if len(pushed) == 0 || !g.preflight(stderr) {
		return
	}
	sort.Slice(pushed, func(i, j int) bool { return pushed[i].name < pushed[j].name })
	sort.Slice(notPushed, func(i, j int) bool { return notPushed[i].name < notPushed[j].name })

	var paths []string
	var b strings.Builder
	fmt.Fprintf(&b, "push to %s at %s\n\n", host, now.UTC().Format(time.RFC3339))
	for _, r := range pushed {
		paths = append(paths, r.files...)
		fmt.Fprintf(&b, "%s: %s\n", r.name, r.summary)
	}
	if len(notPushed) > 0 {
		b.WriteString("\n")
		for _, r := range notPushed {
			reason := r.reason
			if r.status == pushSkipped {
				reason = r.status
			}
			fmt.Fprintf(&b, "not pushed: %s (%s)\n", r.name, reason)
		}
	}
	paths = append(paths, g.mirrorFiles()...)
	if g.commitPaths(paths, b.String(), stderr) == 0 {
		return
	}
	if len(failedKeys) > 0 {
		fmt.Fprintf(stdout, "jirafs push: committed %d of %d; failed: %s\n", len(pushed), attempted, strings.Join(failedKeys, ", "))
		return
	}
	fmt.Fprintf(stdout, "jirafs push: committed %d issue(s) to git\n", len(pushed))
}
