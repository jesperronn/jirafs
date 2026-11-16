package jira

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// Entity retains the ID and name of a Jira status, type or fix version.
type Entity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Project identifies the Jira project independently of a product name.
type Project struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// IssueRef is shallow: following a link requires an explicit GetIssue call.
type IssueRef struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// Link's direction and label are relative to the fetched issue.
type Link struct {
	ID        string   `json:"id"`
	Type      Entity   `json:"type"`
	Direction string   `json:"direction"`
	Label     string   `json:"label"`
	Target    IssueRef `json:"target"`
}

// Issue is a read model, shared by stories, activities and other issue types.
// Description preserves Data Center wiki text as JSON without rendering it.
type Issue struct {
	// Fields retains the returned Jira field IDs and raw values, including fields
	// represented by normalized projections below. Absent keys remain absent;
	// null, empty containers, nested metadata and JSON numbers are preserved.
	Fields map[string]json.RawMessage `json:"fields"`
	// FieldNames contains Jira's expanded field-ID/display-name mapping, if given.
	FieldNames   map[string]string          `json:"field_names,omitempty"`
	ID           string                     `json:"id"`
	Key          string                     `json:"key"`
	Project      Project                    `json:"project"`
	Type         Entity                     `json:"type"`
	Summary      string                     `json:"summary"`
	Status       Entity                     `json:"status"`
	Description  json.RawMessage            `json:"description"`
	Updated      string                     `json:"updated"`
	FixVersions  []Entity                   `json:"fix_versions"`
	Links        []Link                     `json:"links"`
	CustomFields map[string]json.RawMessage `json:"custom_fields"`
}

// GetIssue reads GET /rest/api/2/issue/{issueIdOrKey} on Jira Data Center.
// It does not recursively fetch links or interpret deployment dependencies.
func (c *Client) GetIssue(ctx context.Context, key string) (Issue, error) {
	if err := validateIssueKey(key); err != nil {
		return Issue{}, err
	}
	body, err := c.get(ctx, c.issueURL(key))
	if err != nil {
		return Issue{}, err
	}
	return decodeIssue(body, c.customFields)
}

func validateIssueKey(key string) error {
	if strings.TrimSpace(key) == "" || key == "." || key == ".." || strings.ContainsAny(key, "\r\n\x00") {
		return errors.New("Jira issue key or ID must be nonempty and contain no control characters")
	}
	return nil
}

func decodeIssue(body []byte, customFields []string) (Issue, error) {
	var raw struct {
		ID     string                     `json:"id"`
		Key    string                     `json:"key"`
		Fields map[string]json.RawMessage `json:"fields"`
		Names  map[string]string          `json:"names"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.ID == "" || raw.Key == "" || raw.Fields == nil {
		return Issue{}, errors.New("invalid Jira issue JSON response")
	}
	// Decode typed fields together while retaining custom values as raw JSON.
	var fields struct {
		Project     Project         `json:"project"`
		Type        Entity          `json:"issuetype"`
		Summary     string          `json:"summary"`
		Status      Entity          `json:"status"`
		Description json.RawMessage `json:"description"`
		Updated     string          `json:"updated"`
		FixVersions []Entity        `json:"fixVersions"`
		Links       []struct {
			ID   string `json:"id"`
			Type struct {
				Entity
				Inward  string `json:"inward"`
				Outward string `json:"outward"`
			} `json:"type"`
			Inward  *IssueRef `json:"inwardIssue"`
			Outward *IssueRef `json:"outwardIssue"`
		} `json:"issuelinks"`
	}
	fieldJSON, err := json.Marshal(raw.Fields)
	if err != nil {
		return Issue{}, errors.New("invalid Jira issue fields")
	}
	if err := json.Unmarshal(fieldJSON, &fields); err != nil {
		return Issue{}, errors.New("invalid Jira issue fields")
	}
	i := Issue{
		Fields: raw.Fields, FieldNames: raw.Names,
		ID: raw.ID, Key: raw.Key, Project: fields.Project, Type: fields.Type,
		Summary: fields.Summary, Status: fields.Status, Description: fields.Description,
		Updated: fields.Updated, FixVersions: fields.FixVersions,
		Links: make([]Link, 0, len(fields.Links)), CustomFields: make(map[string]json.RawMessage),
	}
	for _, link := range fields.Links {
		if link.Inward != nil {
			i.Links = append(i.Links, Link{ID: link.ID, Type: link.Type.Entity, Direction: "inward", Label: link.Type.Inward, Target: *link.Inward})
		}
		if link.Outward != nil {
			i.Links = append(i.Links, Link{ID: link.ID, Type: link.Type.Entity, Direction: "outward", Label: link.Type.Outward, Target: *link.Outward})
		}
	}
	for _, id := range customFields {
		if value, ok := raw.Fields[id]; ok {
			i.CustomFields[id] = value
		}
	}
	return i, nil
}
