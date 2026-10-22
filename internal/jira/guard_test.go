package jira

import (
	"context"
	"strings"
	"testing"
)

func TestNewJiraClientRefusesNonLoopback(t *testing.T) {
	c := NewJiraClient("https://example.com")
	_, err := c.FetchIssue(context.Background(), "ABC-1")
	if err == nil || !strings.Contains(err.Error(), "netguard") {
		t.Fatalf("want netguard error, got %v", err)
	}
}
