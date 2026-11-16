package jira

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// FixVersionChange adds or removes one existing version by canonical positive ID.
// It never replaces the issue's version set or creates/deletes a project version.
type FixVersionChange struct {
	Operation string
	VersionID string
}

// MarshalJSON builds Jira's server-side update operation without a fields/set
// payload. Jira owns project membership, permissions and operation availability.
func (change FixVersionChange) MarshalJSON() ([]byte, error) {
	if change.Operation != "add" && change.Operation != "remove" {
		return nil, errors.New("fix-version operation must be add or remove")
	}
	if !linkIdentifier.MatchString(change.VersionID) {
		return nil, errors.New("version ID must be a canonical positive numeric identifier")
	}
	if _, err := strconv.ParseUint(change.VersionID, 10, 63); err != nil {
		return nil, errors.New("version ID exceeds supported Jira identifier range")
	}
	type version struct {
		ID string `json:"id"`
	}
	return json.Marshal(struct {
		Update map[string][]map[string]version `json:"update"`
	}{map[string][]map[string]version{"fixVersions": {{change.Operation: {ID: change.VersionID}}}}})
}

// ChangeFixVersion submits one server-side add/remove without reading or replacing
// the collection. It shares the single-attempt and unknown-outcome write contract.
func (c *Client) ChangeFixVersion(ctx context.Context, key string, change FixVersionChange) error {
	if err := validateIssueKey(key); err != nil {
		return err
	}
	body, err := json.Marshal(change)
	if err != nil {
		return err
	}
	return c.write(ctx, http.MethodPut, c.apiURL("/rest/api/2/issue/"+url.PathEscape(key)), body, http.StatusNoContent)
}
