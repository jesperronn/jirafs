package cli

import (
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
)

// newJiraClient is the single place that builds a Jira client for the active
// project: it resolves the instance credentials and applies the per-project
// epic link and custom field settings. Every command's client factory
// defaults to it; tests may replace the per-command factory variables.
func newJiraClient(settings *config.Settings, ctx *context.Context, cwd string) (jira.Client, error) {
	creds, err := settings.ResolveInstanceCredentials(ctx.Instance)
	if err != nil {
		return nil, err
	}
	opts := []jira.Option{jira.WithEpicLinkField(jira.ResolveEpicLinkField(projectEpicLinkField(settings, ctx.Name)))}
	if settings != nil {
		opts = append(opts, jira.WithCustomFields(settings.Projects[ctx.Name].CustomFields))
	}
	client := jira.NewJiraClient(creds.BaseURL, opts...)
	client.SetCredentials(creds)
	return client, nil
}
