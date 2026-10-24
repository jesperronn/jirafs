package jira

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
	"sort"
	"strings"
)

// AllowedValue is one permitted value of a create-meta field.
type AllowedValue struct {
	ID    string
	Value string
	Name  string
}

// Label returns the human-readable name of the value.
func (a AllowedValue) Label() string {
	if a.Value != "" {
		return a.Value
	}
	if a.Name != "" {
		return a.Name
	}
	return a.ID
}

// CreateMetaField describes one field accepted when creating an issue.
type CreateMetaField struct {
	ID            string
	Name          string
	Required      bool
	HasDefault    bool
	AllowedValues []AllowedValue
}

type rawMetaField struct {
	FieldID       string `json:"fieldId"`
	Name          string `json:"name"`
	Required      bool   `json:"required"`
	HasDefault    bool   `json:"hasDefaultValue"`
	AllowedValues []struct {
		ID    string `json:"id"`
		Value string `json:"value"`
		Name  string `json:"name"`
	} `json:"allowedValues"`
}

func (r rawMetaField) field(id string) CreateMetaField {
	if id == "" {
		id = r.FieldID
	}
	f := CreateMetaField{ID: id, Name: r.Name, Required: r.Required, HasDefault: r.HasDefault}
	for _, a := range r.AllowedValues {
		f.AllowedValues = append(f.AllowedValues, AllowedValue{ID: a.ID, Value: a.Value, Name: a.Name})
	}
	return f
}

func (c *JiraClient) getJSON(ctx context.Context, url string, target interface{}) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, NewTransportError("cannot create request: " + err.Error())
	}
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return 0, NewUnknownErr("cannot authenticate request: " + err.Error())
	}
	resp, err := c.do(req)
	if err != nil {
		return 0, newTransportCause("request failed: ", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return resp.StatusCode, NewAuthError(fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	if resp.StatusCode == http.StatusNotFound {
		return resp.StatusCode, NewNotFoundError(url)
	}
	if resp.StatusCode >= 400 {
		return resp.StatusCode, mapHTTPErr(resp)
	}
	return resp.StatusCode, decodeJSONResponse(resp, target, "createmeta")
}

// FetchCreateMeta returns the fields accepted when creating an issue of the
// given type in the project. It uses the classic createmeta endpoint and falls
// back to the Jira 9+ /createmeta/{project}/issuetypes/{id} variant on 404.
func (c *JiraClient) FetchCreateMeta(ctx context.Context, projectKey, issueType string) ([]CreateMetaField, error) {
	q := fmt.Sprintf("%s/rest/api/2/issue/createmeta?projectKeys=%s&issuetypeNames=%s&expand=projects.issuetypes.fields",
		c.baseURL, neturl.QueryEscape(projectKey), neturl.QueryEscape(issueType))
	var legacy struct {
		Projects []struct {
			IssueTypes []struct {
				Name   string                  `json:"name"`
				Fields map[string]rawMetaField `json:"fields"`
			} `json:"issuetypes"`
		} `json:"projects"`
	}
	_, err := c.getJSON(ctx, q, &legacy)
	if err == nil {
		var out []CreateMetaField
		for _, p := range legacy.Projects {
			for _, it := range p.IssueTypes {
				if !strings.EqualFold(it.Name, issueType) {
					continue
				}
				for id, f := range it.Fields {
					out = append(out, f.field(id))
				}
			}
		}
		sortMetaFields(out)
		return out, nil
	}
	if !IsClientError(err, ErrNotFound) {
		return nil, err
	}

	base := fmt.Sprintf("%s/rest/api/2/issue/createmeta/%s/issuetypes", c.baseURL, neturl.PathEscape(projectKey))
	var types struct {
		Values     []struct{ ID, Name string } `json:"values"`
		IssueTypes []struct{ ID, Name string } `json:"issueTypes"`
	}
	if _, err := c.getJSON(ctx, base, &types); err != nil {
		return nil, err
	}
	id := ""
	for _, t := range append(types.Values, types.IssueTypes...) {
		if strings.EqualFold(t.Name, issueType) {
			id = t.ID
			break
		}
	}
	if id == "" {
		return nil, NewNotFoundError("issue type " + issueType + " in project " + projectKey)
	}
	var fields struct {
		Values []rawMetaField `json:"values"`
		Fields []rawMetaField `json:"fields"`
	}
	if _, err := c.getJSON(ctx, base+"/"+neturl.PathEscape(id), &fields); err != nil {
		return nil, err
	}
	var out []CreateMetaField
	for _, f := range append(fields.Values, fields.Fields...) {
		out = append(out, f.field(""))
	}
	sortMetaFields(out)
	return out, nil
}

func sortMetaFields(f []CreateMetaField) {
	sort.Slice(f, func(i, j int) bool { return f[i].ID < f[j].ID })
}
