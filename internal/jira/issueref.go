package jira

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var issueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[0-9]+$`)

// ParseIssueRef extracts an issue key from arg. It accepts a bare key
// (PROJ-123), a browse URL (<base>/browse/KEY), or any URL on the base host
// whose query string carries selectedIssue=KEY. URLs on a different host than
// baseURL are rejected.
func ParseIssueRef(arg string, baseURL string) (key string, err error) {
	arg = strings.TrimSpace(arg)
	if issueKeyPattern.MatchString(arg) {
		return arg, nil
	}
	if !strings.Contains(arg, "://") {
		return "", fmt.Errorf("%q is not an issue key or Jira URL; try: jirafs export issue KEY --project <name>", arg)
	}
	u, perr := url.Parse(arg)
	if perr != nil || u.Host == "" {
		return "", fmt.Errorf("cannot parse %q as a Jira URL; try: jirafs export issue KEY --project <name>", arg)
	}
	base, perr := url.Parse(strings.TrimSpace(baseURL))
	if perr != nil || base.Host == "" {
		return "", fmt.Errorf("invalid base_url %q; try: jirafs export issue KEY --project <name>", baseURL)
	}
	if !strings.EqualFold(u.Host, base.Host) {
		return "", fmt.Errorf("URL host %q does not match project base_url host %q; try: jirafs export issue KEY --project <name>", u.Host, base.Host)
	}
	if k := u.Query().Get("selectedIssue"); issueKeyPattern.MatchString(k) {
		return k, nil
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "browse" && issueKeyPattern.MatchString(segs[i+1]) {
			return segs[i+1], nil
		}
	}
	return "", fmt.Errorf("no issue key found in %q; try: jirafs export issue KEY --project <name>", arg)
}
