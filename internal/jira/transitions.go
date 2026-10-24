package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"sort"
)

// Transition is a workflow transition available for an issue.
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		Name string `json:"name"`
	} `json:"to"`
	// Fields are the screen fields of the transition (expand=transitions.fields).
	Fields map[string]TransitionField `json:"fields,omitempty"`
}

// TransitionField describes one field on a transition screen.
type TransitionField struct {
	Required bool   `json:"required"`
	Name     string `json:"name"`
}

// RequiredField is a required transition field with its id.
type RequiredField struct{ ID, Name string }

// RequiredFields returns the required screen fields, sorted by id.
func (t Transition) RequiredFields() []RequiredField {
	var out []RequiredField
	for id, f := range t.Fields {
		if f.Required {
			out = append(out, RequiredField{ID: id, Name: f.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// FetchTransitions lists the transitions currently available for the issue
// via GET /rest/api/2/issue/{key}/transitions. It is read-only.
func (c *JiraClient) FetchTransitions(ctx context.Context, key string) ([]Transition, error) {
	if key == "" {
		return nil, NewNotFoundError("empty key")
	}
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions?expand=transitions.fields", c.baseURL, neturl.PathEscape(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create transitions request: " + err.Error())
	}
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return nil, NewUnknownErr("cannot authenticate transitions request: " + err.Error())
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, newTransportCause("transitions request failed: ", err)
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
func (c *JiraClient) DoTransition(ctx context.Context, key, transitionID string, fields map[string]interface{}) error {
	if key == "" {
		return NewNotFoundError("empty key")
	}
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.baseURL, neturl.PathEscape(key))
	payload := map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	}
	if len(fields) > 0 {
		payload["fields"] = fields
	}
	body, err := json.Marshal(payload)
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
	resp, err := c.do(req)
	if err != nil {
		return newTransportCause("transition request failed: ", err)
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
