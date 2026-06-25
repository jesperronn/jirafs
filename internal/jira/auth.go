package jira

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/jirafs/jirafs/internal/config"
)

// AuthTypeBasic indicates Basic authentication (username/password).
const AuthTypeBasic = "basic"

// AuthTypeAtlassianAPI indicates Atlassian API token authentication.
const AuthTypeAtlassianAPI = "atlassian_api_token"

// AuthTypeBearerToken indicates bearer token authentication.
const AuthTypeBearerToken = "bearer_token"

// AuthTypeOAuth1 indicates OAuth 1.0a authentication.
const AuthTypeOAuth1 = "oauth1"

// BuildAuthenticatedRequest applies the appropriate authentication headers
// to req based on the provided credentials. It supports basic,
// atlassian_api_token, bearer_token, and oauth1 auth types. Unsupported or
// missing required fields return an error.
//
// The function mutates req in place and returns the same request pointer for
// convenience.
func BuildAuthenticatedRequest(req *http.Request, creds config.ResolvedInstanceCredentials) (*http.Request, error) {
	if req == nil {
		return nil, fmt.Errorf("jira: BuildAuthenticatedRequest: nil request")
	}

	switch creds.AuthType {
	case AuthTypeBasic:
		return applyBasicAuth(req, creds.Credential)
	case AuthTypeAtlassianAPI:
		return applyAPITokenAuth(req, creds.Credential)
	case AuthTypeBearerToken:
		return applyBearerTokenAuth(req, creds.Credential)
	case AuthTypeOAuth1:
		return nil, fmt.Errorf("jira: BuildAuthenticatedRequest: oauth1 auth not yet implemented")
	default:
		if creds.AuthType == "" {
			return req, nil
		}
		return nil, fmt.Errorf("jira: BuildAuthenticatedRequest: unsupported auth type %q", creds.AuthType)
	}
}

// applyBasicAuth sets the Authorization header using HTTP Basic Auth.
func applyBasicAuth(req *http.Request, cred config.ResolvedCredential) (*http.Request, error) {
	username := cred.Fields["username"]
	password := cred.Fields["password"]

	if username == "" {
		return nil, fmt.Errorf("jira: basic auth requires 'username' field")
	}
	if password == "" {
		return nil, fmt.Errorf("jira: basic auth requires 'password' field")
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Authorization", "Basic "+encoded)
	return req, nil
}

// applyAPITokenAuth sets the Authorization header using Atlassian API token
// format: Basic base64(email:api_token).
func applyAPITokenAuth(req *http.Request, cred config.ResolvedCredential) (*http.Request, error) {
	email := cred.Fields["email"]
	apiToken := cred.Fields["api_token"]

	if email == "" {
		return nil, fmt.Errorf("jira: atlassian_api_token auth requires 'email' field")
	}
	if apiToken == "" {
		return nil, fmt.Errorf("jira: atlassian_api_token auth requires 'api_token' field")
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(email + ":" + apiToken))
	req.Header.Set("Authorization", "Basic "+encoded)
	return req, nil
}

func applyBearerTokenAuth(req *http.Request, cred config.ResolvedCredential) (*http.Request, error) {
	token := cred.Fields["bearer_token"]
	if token == "" {
		return nil, fmt.Errorf("jira: bearer_token auth requires 'bearer_token' field")
	}

	req.Header.Set("Authorization", "Bearer "+token)
	return req, nil
}
