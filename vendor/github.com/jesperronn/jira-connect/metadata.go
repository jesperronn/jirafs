package jira

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// GetProject returns the raw project object for a project key or ID.
func (c *Client) GetProject(ctx context.Context, key string) (json.RawMessage, error) {
	if err := validateMetadataProject(key); err != nil {
		return nil, err
	}
	return c.metadataObject(ctx, "/rest/api/2/project/"+url.PathEscape(key))
}

// CurrentUser returns the authenticated user's raw Data Center user object.
func (c *Client) CurrentUser(ctx context.Context) (json.RawMessage, error) {
	return c.metadataObject(ctx, "/rest/api/2/myself")
}

// ListFields returns the raw field catalogue; presence does not imply editability.
func (c *Client) ListFields(ctx context.Context) ([]json.RawMessage, error) {
	body, err := c.get(ctx, c.apiURL("/rest/api/2/field"))
	if err != nil {
		return nil, err
	}
	var values []json.RawMessage
	if json.Unmarshal(body, &values) != nil || values == nil {
		return nil, errors.New("invalid Jira fields array response")
	}
	for _, value := range values {
		if !metadataIsObject(value) {
			return nil, errors.New("invalid Jira fields array response")
		}
	}
	return values, nil
}

// ListIssueTypes returns every createable issue type's raw object, including its
// id and name. Callers resolve display names to IDs before requesting fields.
func (c *Client) ListIssueTypes(ctx context.Context, projectKey string) ([]json.RawMessage, error) {
	if err := validateMetadataProject(projectKey); err != nil {
		return nil, err
	}
	return c.metadataPages(ctx, "/rest/api/2/issue/createmeta/"+url.PathEscape(projectKey)+"/issuetypes", "id")
}

// CreateMetadata returns all raw field metadata for an explicit issue type ID.
// This reads metadata only; it does not create an issue or resolve a type name.
func (c *Client) CreateMetadata(ctx context.Context, projectKey, issueTypeID string) ([]json.RawMessage, error) {
	if err := validateMetadataProject(projectKey); err != nil {
		return nil, err
	}
	if !linkIdentifier.MatchString(issueTypeID) {
		return nil, errors.New("issue type ID must be a canonical positive numeric identifier")
	}
	if _, err := strconv.ParseUint(issueTypeID, 10, 63); err != nil {
		return nil, errors.New("issue type ID exceeds supported Jira identifier range")
	}
	return c.metadataPages(ctx, "/rest/api/2/issue/createmeta/"+url.PathEscape(projectKey)+"/issuetypes/"+url.PathEscape(issueTypeID), "fieldId")
}

// EditMetadata returns the raw edit metadata object for an existing issue.
func (c *Client) EditMetadata(ctx context.Context, key string) (json.RawMessage, error) {
	if err := validateIssueKey(key); err != nil {
		return nil, err
	}
	return c.metadataObject(ctx, "/rest/api/2/issue/"+url.PathEscape(key)+"/editmeta")
}

func validateMetadataProject(key string) error {
	if strings.TrimSpace(key) == "" || key == "." || key == ".." || strings.ContainsAny(key, "\r\n\x00") {
		return errors.New("Jira project key must be nonempty and contain no control characters")
	}
	return nil
}

func metadataIsObject(value []byte) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(value, &object) == nil && object != nil
}

func (c *Client) metadataObject(ctx context.Context, path string) (json.RawMessage, error) {
	body, err := c.get(ctx, c.apiURL(path))
	if err != nil {
		return nil, err
	}
	if !metadataIsObject(body) {
		return nil, errors.New("invalid Jira metadata object response")
	}
	return json.RawMessage(body), nil
}

// Both Data Center createmeta resources use values pages. Advance by actual
// returned count because Jira may cap maxResults below the requested limit.
func (c *Client) metadataPages(ctx context.Context, path, identity string) ([]json.RawMessage, error) {
	const pageSize = 100
	const maxPages = 1000
	const maxItems = 100000
	values := make([]json.RawMessage, 0)
	seen := make(map[string]bool)
	start := 0
	for range maxPages {
		u := c.apiURL(path)
		u.RawQuery = url.Values{"startAt": {strconv.Itoa(start)}, "maxResults": {strconv.Itoa(pageSize)}}.Encode()
		body, err := c.get(ctx, u)
		if err != nil {
			return nil, err
		}
		var page struct {
			Start  *int              `json:"startAt"`
			Max    *int              `json:"maxResults"`
			Total  *int              `json:"total"`
			Last   *bool             `json:"isLast"`
			Values []json.RawMessage `json:"values"`
		}
		invalid := errors.New("invalid Jira metadata page response")
		if json.Unmarshal(body, &page) != nil || page.Start == nil || page.Max == nil || page.Total == nil || page.Values == nil {
			return nil, invalid
		}
		if *page.Start != start || *page.Max < 0 || *page.Total < start || len(page.Values) > *page.Max || len(page.Values) > *page.Total-start {
			return nil, invalid
		}
		if len(values)+len(page.Values) > maxItems {
			return nil, errors.New("Jira metadata exceeds 100000 item limit")
		}
		for _, value := range page.Values {
			var object map[string]json.RawMessage
			if json.Unmarshal(value, &object) != nil || object == nil {
				return nil, invalid
			}
			var id string
			if json.Unmarshal(object[identity], &id) != nil || id == "" || seen[id] {
				return nil, errors.New("invalid or duplicate Jira metadata identity")
			}
			if identity == "id" {
				var name string
				if json.Unmarshal(object["name"], &name) != nil || name == "" {
					return nil, invalid
				}
			}
			seen[id] = true
		}
		next := start + len(page.Values)
		last := next == *page.Total
		if page.Last != nil && *page.Last != last {
			return nil, invalid
		}
		if !last && len(page.Values) == 0 {
			return nil, errors.New("Jira metadata pagination made no progress")
		}
		values = append(values, page.Values...)
		if last {
			return values, nil
		}
		start = next
	}
	return nil, errors.New("Jira metadata exceeds 1000 page limit")
}
