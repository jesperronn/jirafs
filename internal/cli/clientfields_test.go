package cli

import (
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/jira"
)

func TestConfigureClientFields(t *testing.T) {
	s := &config.Settings{Projects: map[string]config.Project{"p": {CustomFields: []string{"customfield_1"}}}}
	configureClientFields(jira.NewJiraClient("http://x"), s, "p")
	configureClientFields(jira.NewJiraClient("http://x"), nil, "p")
}
