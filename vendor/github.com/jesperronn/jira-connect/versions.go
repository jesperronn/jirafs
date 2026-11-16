package jira

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// Version is a project-scoped fix version, not evidence of deployment.
// Dates retain Jira's YYYY-MM-DD strings; an absent date is empty.
type Version struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Project     Project `json:"project"`
	Description string  `json:"description"`
	Released    bool    `json:"released"`
	Archived    bool    `json:"archived"`
	StartDate   string  `json:"start_date,omitempty"`
	ReleaseDate string  `json:"release_date,omitempty"`
}

// ListVersions reads the full, unpaginated REST v2 version array. It includes
// released, unreleased and archived versions without choosing a latest release.
func (c *Client) ListVersions(ctx context.Context, projectKey string) ([]Version, error) {
	if strings.TrimSpace(projectKey) == "" || projectKey == "." || projectKey == ".." || strings.ContainsAny(projectKey, "\r\n\x00") {
		return nil, errors.New("Jira project key must be nonempty and contain no control characters")
	}
	u := c.apiURL("/rest/api/2/project/" + url.PathEscape(projectKey) + "/versions")
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var raw []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		ProjectID   int64  `json:"projectId"`
		ProjectKey  string `json:"project"`
		Description string `json:"description"`
		Released    bool   `json:"released"`
		Archived    bool   `json:"archived"`
		StartDate   string `json:"startDate"`
		ReleaseDate string `json:"releaseDate"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return nil, errors.New("invalid Jira versions array response")
	}
	versions := make([]Version, 0, len(raw))
	for _, v := range raw {
		if v.ID == "" || v.Name == "" || v.ProjectID < 0 {
			return nil, errors.New("invalid Jira version identity")
		}
		project := Project{Key: projectKey}
		if v.ProjectID != 0 {
			project.ID = strconv.FormatInt(v.ProjectID, 10)
		}
		if v.ProjectKey != "" {
			project.Key = v.ProjectKey
		}
		versions = append(versions, Version{
			ID: v.ID, Name: v.Name, Project: project, Description: v.Description,
			Released: v.Released, Archived: v.Archived,
			StartDate: v.StartDate, ReleaseDate: v.ReleaseDate,
		})
	}
	return versions, nil
}
