package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
)

// Transition is a workflow transition available for an issue.
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		Name string `json:"name"`
	} `json:"to"`
}

// FetchTransitions lists the transitions currently available for the issue
// via GET /rest/api/2/issue/{key}/transitions. It is read-only.
func (c *JiraClient) FetchTransitions(ctx context.Context, key string) ([]Transition, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if key == "" {
		return nil, NewNotFoundError("empty key")
	}
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.baseURL, neturl.PathEscape(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create transitions request: " + err.Error())
	}
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return nil, NewUnknownErr("cannot authenticate transitions request: " + err.Error())
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewTransportError("transitions request failed: " + err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, NewNotFoundError(key)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, NewAuthError(fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	if resp.StatusCode >= 400 {
		return nil, mapHTTPErr(resp)
	}
	var out struct {
		Transitions []Transition `json:"transitions"`
	}
	if err := decodeJSONResponse(resp, &out, "transitions"); err != nil {
		return nil, err
	}
	return out.Transitions, nil
}

// DoTransition performs a transition via POST
// /rest/api/2/issue/{key}/transitions; Jira answers 204 with no body.
func (c *JiraClient) DoTransition(ctx context.Context, key, transitionID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if key == "" {
		return NewNotFoundError("empty key")
	}
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.baseURL, neturl.PathEscape(key))
	body, err := json.Marshal(map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	})
	if err != nil {
		return NewUnknownErr("cannot marshal transition request: " + err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return NewTransportError("cannot create transition request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return NewUnknownErr("cannot authenticate transition request: " + err.Error())
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return NewTransportError("transition request failed: " + err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return NewNotFoundError(key)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return NewAuthError(fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	if resp.StatusCode >= 400 {
		return mapHTTPErr(resp)
	}
	return nil
}
