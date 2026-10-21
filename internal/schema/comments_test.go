package schema

import (
	"fmt"
	"strings"
	"testing"
)

func fixtureComments(n int) []Comment {
	cs := make([]Comment, n)
	for i := range cs {
		// Newest first: index 0 is the newest.
		cs[i] = Comment{
			Author:  fmt.Sprintf("user%d", n-i),
			Created: fmt.Sprintf("2024-03-%02dT10:00:00Z", n-i),
			Body:    fmt.Sprintf("comment body %d", n-i),
		}
	}
	return cs
}

func TestRenderIssue_CommentsNewestFirstAndCap(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{Key: "PROJ-1", Type: "story"},
		Summary:  "With comments",
		Comments: fixtureComments(20),
		// 25 total on the server, 20 shown.
		CommentsTotal: 25,
	}
	out := RenderIssue(issue)
	t.Logf("rendered fixture issue:\n%s", out)

	if !strings.Contains(out, "## Comments\n_(showing 20 of 25 comments)_\n") {
		t.Errorf("missing cap note:\n%s", out)
	}
	if n := strings.Count(out, "\n### "); n != 20 {
		t.Errorf("comment headers = %d, want 20", n)
	}
	newest := strings.Index(out, "### user20 — 2024-03-20T10:00:00Z")
	oldest := strings.Index(out, "### user1 — 2024-03-01T10:00:00Z")
	if newest < 0 || oldest < 0 || newest > oldest {
		t.Errorf("want newest before oldest (newest=%d oldest=%d)", newest, oldest)
	}
	if !strings.Contains(out, "comment body 20\n") {
		t.Errorf("missing body:\n%s", out)
	}
}

func TestRenderIssue_CommentsNoNoteWhenAllShown(t *testing.T) {
	out := RenderIssue(Issue{Comments: fixtureComments(3), CommentsTotal: 3})
	if strings.Contains(out, "showing") {
		t.Errorf("unexpected cap note:\n%s", out)
	}
}

func TestRenderIssue_NoCommentsOmitsSection(t *testing.T) {
	out := RenderIssue(Issue{Summary: "x"})
	if strings.Contains(out, "## Comments") {
		t.Errorf("unexpected comments section:\n%s", out)
	}
}

func TestParseIssue_KeepsCommentsSectionAndRoundTrips(t *testing.T) {
	issue := Issue{
		Identity:      IssueIdentity{Key: "PROJ-1", Type: "story"},
		Summary:       "S",
		Comments:      append(fixtureComments(2), Comment{Author: "x", Created: "2024-01-01T00:00:00Z", Body: "line\n## not a heading"}),
		CommentsTotal: 2,
	}
	out := RenderIssue(issue)
	parsed, pe := ParseIssue(out)
	if pe != nil {
		t.Fatalf("ParseIssue: %v", pe)
	}
	if !strings.Contains(parsed.Sections[SecComments], "### user2 — 2024-03-02T10:00:00Z") {
		t.Errorf("comments section not kept: %q", parsed.Sections[SecComments])
	}
	// Re-rendering a parsed file preserves the section verbatim.
	again := RenderIssue(parsed)
	if !strings.Contains(again, "## Comments\n### user2") && !strings.Contains(again, "## Comments\n\n### user2") {
		t.Errorf("re-render lost comments:\n%s", again)
	}
	if !strings.Contains(again, "\\## not a heading") {
		t.Errorf("re-render lost escaped line:\n%s", again)
	}
}

func TestFixedSectionName_CommentsKnownButNotCanonical(t *testing.T) {
	if !SecComments.IsKnown() {
		t.Error("SecComments should be known")
	}
	for _, s := range AllFixedSections() {
		if s == SecComments {
			t.Error("SecComments must not be in AllFixedSections")
		}
	}
}
