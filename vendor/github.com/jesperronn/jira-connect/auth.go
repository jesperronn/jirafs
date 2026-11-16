package jira

import (
	"errors"
	"net/http"
	"strings"
)

// AuthType selects a Jira Data Center authentication scheme.
type AuthType string

const (
	BasicAuth  AuthType = "basic"
	BearerAuth AuthType = "bearer_token"
)

// Credentials contains resolved credentials. Reference resolution belongs to
// the caller; the client never loads settings or invokes credential tools.
type Credentials struct {
	Type     AuthType
	Username string
	Secret   string
}

func (c Credentials) validate() error {
	if c.Secret == "" || strings.ContainsAny(c.Secret, "\r\n") {
		return errors.New("Jira credentials require a nonempty secret without line breaks")
	}
	switch c.Type {
	case BasicAuth:
		if strings.TrimSpace(c.Username) == "" || strings.ContainsAny(c.Username, ":\r\n") {
			return errors.New("Jira basic authentication requires a username without colons or line breaks")
		}
	case BearerAuth:
	default:
		return errors.New("Jira authentication must be basic or bearer_token")
	}
	return nil
}

func (c Credentials) authenticate(r *http.Request) {
	if c.Type == BasicAuth {
		r.SetBasicAuth(c.Username, c.Secret)
	} else {
		r.Header.Set("Authorization", "Bearer "+c.Secret)
	}
}
