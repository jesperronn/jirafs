package cli

import (
	"fmt"
	"os"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/jiraconnect"
)

// newJiraClient is the single place that builds a Jira client for the active
// project: it resolves the instance credentials and applies the per-project
// epic link and custom field settings. Every command's client factory
// defaults to it; tests may replace the per-command factory variables.
func newJiraClient(settings *config.Settings, ctx *context.Context, cwd string) (jira.Client, error) {
	adapter := os.Getenv("JIRAFS_JIRA_ADAPTER")
	if adapter != "" && adapter != "legacy" && adapter != "jira-connect" {
		return nil, fmt.Errorf("JIRAFS_JIRA_ADAPTER must be legacy or jira-connect")
	}
	creds, err := settings.ResolveInstanceCredentials(ctx.Instance)
	if err != nil {
		return nil, err
	}
	if adapter == "jira-connect" {
		client := jiraconnect.New(jiraconnect.Config{
			BaseURL:       creds.BaseURL,
			EpicLinkField: jira.ResolveEpicLinkField(projectEpicLinkField(settings, ctx.Name)),
			CustomFields:  settings.Projects[ctx.Name].CustomFields,
		})
		client.SetCredentials(creds)
		return client, nil
	}

	opts := []jira.Option{jira.WithEpicLinkField(jira.ResolveEpicLinkField(projectEpicLinkField(settings, ctx.Name)))}
	if settings != nil {
		opts = append(opts, jira.WithCustomFields(settings.Projects[ctx.Name].CustomFields))
	}
	client := jira.NewJiraClient(creds.BaseURL, opts...)
	client.SetCredentials(creds)
	return client, nil
}
