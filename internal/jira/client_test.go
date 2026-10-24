package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/schema"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestNewJiraClient(t *testing.T) {
	client := NewJiraClient("https://jira.example.com")
	if client.baseURL != "https://jira.example.com" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "https://jira.example.com")
	}
	if client.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
}

func TestFetchIssueSuccess(t *testing.T) {
	payload := map[string]interface{}{
		"id":  "10001",
		"key": "PROJ-123",
		"fields": map[string]interface{}{
			"issuetype": map[string]interface{}{
				"name": "Story",
			},
			"summary": "Test issue",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		wantPath := "/rest/api/2/issue/PROJ-123"
		if r.URL.Path != wantPath {
			t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	issue, err := client.FetchIssue(context.Background(), "PROJ-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected non-nil issue")
	}
	if issue.Identity.Key != "PROJ-123" {
		t.Errorf("key = %q, want %q", issue.Identity.Key, "PROJ-123")
	}
}

func TestFetchIssueNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errorMessages":["Issue does not exist"],"errors":{}}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchIssue(context.Background(), "PROJ-999")
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestFetchIssueHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errorMessages":["Internal server error"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchIssue(context.Background(), "PROJ-123")
	if !IsClientError(err, ErrHTTP) {
		t.Errorf("expected http error, got %v", err)
	}
}

func TestFetchIssueAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchIssue(context.Background(), "PROJ-123")
	if !IsClientError(err, ErrAuth) {
		t.Errorf("expected auth error, got %v", err)
	}
}

func TestFetchIssueEmptyKey(t *testing.T) {
	client := NewJiraClient("https://jira.example.com")
	_, err := client.FetchIssue(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestFetchIssueTransportError(t *testing.T) {
	client := NewJiraClient("http://localhost:1")                         // unlikely to be listening
	ctx, cancel := context.WithTimeout(context.Background(), 100*1000000) // 100ms
	defer cancel()
	_, err := client.FetchIssue(ctx, "PROJ-123")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestJiraClientImplementsClient(t *testing.T) {
	var _ Client = (*JiraClient)(nil)
}

func TestFetchIssueWithAssignee(t *testing.T) {
	payload := map[string]interface{}{
		"id":  "10002",
		"key": "PROJ-456",
		"fields": map[string]interface{}{
			"issuetype": map[string]interface{}{
				"name": "Bug",
			},
			"summary": "Bug issue",
			"assignee": map[string]interface{}{
				"name":        "jdoe",
				"displayName": "Jane Doe",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	issue, err := client.FetchIssue(context.Background(), "PROJ-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected non-nil issue")
	}
}

func TestFetchIssueWithLabels(t *testing.T) {
	payload := map[string]interface{}{
		"id":  "10003",
		"key": "PROJ-789",
		"fields": map[string]interface{}{
			"issuetype": map[string]interface{}{
				"name": "Task",
			},
			"summary": "Task with labels",
			"labels":  []string{"urgent", "blocked"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	issue, err := client.FetchIssue(context.Background(), "PROJ-789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected non-nil issue")
	}
}

func TestFetchIssueWithNoIssueType(t *testing.T) {
	payload := map[string]interface{}{
		"id":  "10004",
		"key": "PROJ-000",
		"fields": map[string]interface{}{
			"summary": "Issue without type",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	issue, err := client.FetchIssue(context.Background(), "PROJ-000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected non-nil issue")
	}
	if issue.Identity.Type != "" {
		t.Errorf("expected empty type, got %q", issue.Identity.Type)
	}
}

func TestMapHTTPErrWithErrorMessage(t *testing.T) {
	body := `{"errorMessages":["Permission denied","Board not found"],"errors":{}}`
	resp := httptest.NewRecorder()
	resp.Body = nil // will be set by server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchIssue(context.Background(), "PROJ-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsClientError(err, ErrHTTP) {
		t.Errorf("expected http error, got %v", err)
	}
	wantMsg := "Permission denied"
	if !contains(err.Error(), wantMsg) {
		t.Errorf("error should contain %q, got %q", wantMsg, err.Error())
	}
}

func TestMapHTTPErrWithErrorsMap(t *testing.T) {
	body := `{"errorMessages":[],"errors":{"summary":"must not be empty"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchIssue(context.Background(), "PROJ-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsClientError(err, ErrHTTP) {
		t.Errorf("expected http error, got %v", err)
	}
	if !contains(err.Error(), "summary") {
		t.Errorf("error message should mention field 'summary', got %q", err.Error())
	}
}

func TestMapHTTPErrEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		// no body
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchIssue(context.Background(), "PROJ-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsClientError(err, ErrHTTP) {
		t.Errorf("expected http error, got %v", err)
	}
	if err.Error() != "jira: http: HTTP 500 Internal Server Error" {
		t.Errorf("error = %q, want %q", err.Error(), "jira: http: HTTP 500 Internal Server Error")
	}
}

func TestMapHTTPErrEmptyJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchIssue(context.Background(), "PROJ-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsClientError(err, ErrHTTP) {
		t.Errorf("expected http error, got %v", err)
	}
	if err.Error() != "jira: http: HTTP 500 Internal Server Error: {}" {
		t.Errorf("error = %q, want %q", err.Error(), "jira: http: HTTP 500 Internal Server Error: {}")
	}
}

func TestFetchIssueWithErrorMessage(t *testing.T) {
	body := `{"errorMessages":["Issue does not exist"],"errors":{}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchIssue(context.Background(), "PROJ-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsClientError(err, ErrHTTP) {
		t.Errorf("expected http error, got %v", err)
	}
	if !contains(err.Error(), "Issue does not exist") {
		t.Errorf("error should contain Jira message, got %q", err.Error())
	}
}

func TestFetchIssueServerErrorMessage(t *testing.T) {
	body := `{"errorMessages":["Internal server error occurred"],"errors":{}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchIssue(context.Background(), "PROJ-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsClientError(err, ErrHTTP) {
		t.Errorf("expected http error, got %v", err)
	}
	if !contains(err.Error(), "Internal server error occurred") {
		t.Errorf("error should contain Jira message, got %q", err.Error())
	}
}

func TestFetchIssueAuthErrorWithErrorMessage(t *testing.T) {
	body := `{"errorMessages":["You are not authorized to use this credential"],"errors":{}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchIssue(context.Background(), "PROJ-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsClientError(err, ErrAuth) {
		t.Errorf("expected auth error, got %v", err)
	}
}

func TestFetchIssueWithNilFields(t *testing.T) {
	payload := map[string]interface{}{
		"id":     "10005",
		"key":    "PROJ-001",
		"fields": nil,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	issue, err := client.FetchIssue(context.Background(), "PROJ-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected non-nil issue")
	}
	if issue.Identity.Key != "PROJ-001" {
		t.Errorf("key = %q, want %q", issue.Identity.Key, "PROJ-001")
	}
}

func TestBuildAuthenticatedRequestBasicAuth(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		BaseURL:  "https://jira.example.com",
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Scheme: "env",
			Target: "JIRA_CRED",
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	}

	req, err := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	if err != nil {
		t.Fatalf("unexpected error creating request: %v", err)
	}

	out, err := BuildAuthenticatedRequest(req, creds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != req {
		t.Error("expected same request pointer returned")
	}

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:secret"))
	got := out.Header.Get("Authorization")
	if got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestBuildAuthenticatedRequestAPIToken(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		BaseURL:  "https://jira.example.com",
		AuthType: "atlassian_api_token",
		Credential: config.ResolvedCredential{
			Scheme: "env",
			Target: "JIRA_TOKEN",
			Fields: map[string]string{
				"email":     "user@example.com",
				"api_token": "my-api-token",
			},
		},
	}

	req, err := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	if err != nil {
		t.Fatalf("unexpected error creating request: %v", err)
	}

	out, err := BuildAuthenticatedRequest(req, creds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:my-api-token"))
	got := out.Header.Get("Authorization")
	if got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestBuildAuthenticatedRequestBearerToken(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		BaseURL:  "https://jira.example.com",
		AuthType: "bearer_token",
		Credential: config.ResolvedCredential{
			Scheme: "op",
			Target: "JIRA_API_TOKEN_NINE_JRJ",
			Fields: map[string]string{
				"bearer_token": "my-bearer-token",
			},
		},
	}

	req, err := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	if err != nil {
		t.Fatalf("unexpected error creating request: %v", err)
	}

	out, err := BuildAuthenticatedRequest(req, creds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.Header.Get("Authorization")
	if got != "Bearer my-bearer-token" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer my-bearer-token")
	}
}

func TestBuildAuthenticatedRequestEmptyAuthType(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		BaseURL:    "https://jira.example.com",
		AuthType:   "",
		Credential: config.ResolvedCredential{},
	}

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	out, err := BuildAuthenticatedRequest(req, creds)
	if err != nil {
		t.Fatalf("unexpected error for empty auth type: %v", err)
	}
	if out != req {
		t.Error("expected same request pointer returned")
	}
}

func TestBuildAuthenticatedRequestUnsupportedAuthType(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		AuthType: "oauth2",
	}

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	_, err := BuildAuthenticatedRequest(req, creds)
	if err == nil {
		t.Fatal("expected error for unsupported auth type")
	}
	if !contains(err.Error(), "unsupported auth type") {
		t.Errorf("error should mention unsupported auth type, got %q", err.Error())
	}
}

func TestBuildAuthenticatedRequestBasicMissingUsername(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{"password": "secret"},
		},
	}

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	_, err := BuildAuthenticatedRequest(req, creds)
	if err == nil {
		t.Fatal("expected error for missing username")
	}
	if !contains(err.Error(), "username") {
		t.Errorf("error should mention username, got %q", err.Error())
	}
}

func TestBuildAuthenticatedRequestBasicMissingPassword(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{"username": "user@example.com"},
		},
	}

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	_, err := BuildAuthenticatedRequest(req, creds)
	if err == nil {
		t.Fatal("expected error for missing password")
	}
	if !contains(err.Error(), "password") {
		t.Errorf("error should mention password, got %q", err.Error())
	}
}

func TestBuildAuthenticatedRequestAPITokenMissingEmail(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		AuthType: "atlassian_api_token",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{"api_token": "my-token"},
		},
	}

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	_, err := BuildAuthenticatedRequest(req, creds)
	if err == nil {
		t.Fatal("expected error for missing email")
	}
	if !contains(err.Error(), "email") {
		t.Errorf("error should mention email, got %q", err.Error())
	}
}

func TestBuildAuthenticatedRequestAPITokenMissingToken(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		AuthType: "atlassian_api_token",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{"email": "user@example.com"},
		},
	}

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	_, err := BuildAuthenticatedRequest(req, creds)
	if err == nil {
		t.Fatal("expected error for missing api_token")
	}
	if !contains(err.Error(), "api_token") {
		t.Errorf("error should mention api_token, got %q", err.Error())
	}
}

func TestBuildAuthenticatedRequestBearerTokenMissingToken(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		AuthType:   "bearer_token",
		Credential: config.ResolvedCredential{Fields: map[string]string{}},
	}

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	_, err := BuildAuthenticatedRequest(req, creds)
	if err == nil {
		t.Fatal("expected error for missing bearer_token")
	}
	if !contains(err.Error(), "bearer_token") {
		t.Errorf("error should mention bearer_token, got %q", err.Error())
	}
}

func TestBuildAuthenticatedRequestNilRequest(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{"username": "u", "password": "p"},
		},
	}

	_, err := BuildAuthenticatedRequest(nil, creds)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
	if !contains(err.Error(), "nil request") {
		t.Errorf("error should mention nil request, got %q", err.Error())
	}
}

func TestCurrentUserSuccess(t *testing.T) {
	payload := map[string]interface{}{
		"self":         "https://jira.example.com/rest/api/2/user?username=john.doe",
		"key":          "john.doe",
		"name":         "john.doe",
		"emailAddress": "john@example.com",
		"displayName":  "John Doe",
		"active":       true,
		"timeZone":     "America/New_York",
		"accountType":  "atlassian",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		wantPath := "/rest/api/2/myself"
		if r.URL.Path != wantPath {
			t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer my-bearer-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer my-bearer-token")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "bearer_token",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"bearer_token": "my-bearer-token",
			},
		},
	})
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.Name != "john.doe" {
		t.Errorf("name = %q, want %q", user.Name, "john.doe")
	}
	if user.DisplayName != "John Doe" {
		t.Errorf("displayName = %q, want %q", user.DisplayName, "John Doe")
	}
	if user.EmailAddress != "john@example.com" {
		t.Errorf("emailAddress = %q, want %q", user.EmailAddress, "john@example.com")
	}
	if !user.Active {
		t.Error("expected active user")
	}
	if user.Timezone != "America/New_York" {
		t.Errorf("timezone = %q, want %q", user.Timezone, "America/New_York")
	}
	if user.AccountType != "atlassian" {
		t.Errorf("accountType = %q, want %q", user.AccountType, "atlassian")
	}
}

func TestCurrentUserParseErrorIncludesBodyPreview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>login</body></html>"))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "bearer_token",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"bearer_token": "my-bearer-token",
			},
		},
	})

	_, err := client.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !contains(err.Error(), "HTTP 200") {
		t.Fatalf("error = %q, want status code", err.Error())
	}
	if !contains(err.Error(), "login") {
		t.Fatalf("error = %q, want body preview", err.Error())
	}
}

func TestCurrentUserNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errorMessages":["User does not exist"],"errors":{}}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.CurrentUser(context.Background())
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestCurrentUserAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.CurrentUser(context.Background())
	if !IsClientError(err, ErrAuth) {
		t.Errorf("expected auth error, got %v", err)
	}
}

func TestCurrentUserHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errorMessages":["Internal server error"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.CurrentUser(context.Background())
	if !IsClientError(err, ErrHTTP) {
		t.Errorf("expected http error, got %v", err)
	}
}

func TestCurrentUserTransportError(t *testing.T) {
	client := NewJiraClient("http://localhost:1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*1000000)
	defer cancel()
	_, err := client.CurrentUser(ctx)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestSearchIssuesMyIssues(t *testing.T) {
	payload := map[string]interface{}{
		"total":      2,
		"maxResults": 50,
		"issues": []map[string]interface{}{
			{
				"id":  "10001",
				"key": "PROJ-10",
				"fields": map[string]interface{}{
					"issuetype": map[string]interface{}{
						"name": "Story",
					},
					"summary": "First issue",
					"labels":  []string{"urgent"},
				},
			},
			{
				"id":  "10002",
				"key": "PROJ-11",
				"fields": map[string]interface{}{
					"issuetype": map[string]interface{}{
						"name": "Bug",
					},
					"summary": "Second issue",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		wantPath := "/rest/api/3/search/jql"
		if r.URL.Path != wantPath {
			t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	issues, total, err := client.SearchIssues(context.Background(), "assignee = currentUser()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if string(issues[0].Identity.Key) != "PROJ-10" {
		t.Errorf("issue 0 key = %q, want %q", issues[0].Identity.Key, "PROJ-10")
	}
	if string(issues[0].Identity.Type) != "Story" {
		t.Errorf("issue 0 type = %q, want %q", issues[0].Identity.Type, "Story")
	}
	if string(issues[1].Identity.Key) != "PROJ-11" {
		t.Errorf("issue 1 key = %q, want %q", issues[1].Identity.Key, "PROJ-11")
	}
	if string(issues[1].Identity.Type) != "Bug" {
		t.Errorf("issue 1 type = %q, want %q", issues[1].Identity.Type, "Bug")
	}
	if len(issues[0].Labels) != 1 || issues[0].Labels[0] != "urgent" {
		t.Errorf("issue 0 labels = %v, want [urgent]", issues[0].Labels)
	}
}

func TestSearchIssuesCurrentSprint(t *testing.T) {
	payload := map[string]interface{}{
		"total":      2,
		"maxResults": 50,
		"issues": []map[string]interface{}{
			{
				"id":  "10003",
				"key": "PROJ-20",
				"fields": map[string]interface{}{
					"issuetype": map[string]interface{}{
						"name": "Story",
					},
					"summary": "Sprint story",
				},
			},
			{
				"id":  "10004",
				"key": "PROJ-21",
				"fields": map[string]interface{}{
					"issuetype": map[string]interface{}{
						"name": "Bug",
					},
					"summary": "Sprint bug",
				},
			},
		},
	}

	sentJQL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/rest/api/3/search/jql" {
			t.Errorf("path = %s, want /rest/api/3/search/jql", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		sentJQL = body["jql"].(string)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	issues, total, err := client.SearchIssues(context.Background(), "sprint in openSprints()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if string(issues[0].Identity.Key) != "PROJ-20" {
		t.Errorf("issue 0 key = %q, want %q", issues[0].Identity.Key, "PROJ-20")
	}
	if string(issues[0].Identity.Type) != "Story" {
		t.Errorf("issue 0 type = %q, want %q", issues[0].Identity.Type, "Story")
	}
	if string(issues[1].Identity.Key) != "PROJ-21" {
		t.Errorf("issue 1 key = %q, want %q", issues[1].Identity.Key, "PROJ-21")
	}
	if string(issues[1].Identity.Type) != "Bug" {
		t.Errorf("issue 1 type = %q, want %q", issues[1].Identity.Type, "Bug")
	}
	wantJQL := "sprint in openSprints()"
	if sentJQL != wantJQL {
		t.Errorf("sent JQL = %q, want %q", sentJQL, wantJQL)
	}
}

func TestSearchIssuesEmptyJQL(t *testing.T) {
	client := NewJiraClient("https://jira.example.com")
	_, _, err := client.SearchIssues(context.Background(), "  ")
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestSearchIssuesAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	_, _, err := client.SearchIssues(context.Background(), "assignee = currentUser()")
	if !IsClientError(err, ErrAuth) {
		t.Errorf("expected auth error, got %v", err)
	}
}

func TestSearchIssuesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errorMessages":["Internal server error"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, _, err := client.SearchIssues(context.Background(), "assignee = currentUser()")
	if !IsClientError(err, ErrHTTP) {
		t.Errorf("expected http error, got %v", err)
	}
}

func TestSearchIssuesTransportError(t *testing.T) {
	client := NewJiraClient("http://localhost:1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*1000000)
	defer cancel()
	_, _, err := client.SearchIssues(ctx, "assignee = currentUser()")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestSearchIssuesInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	_, _, err := client.SearchIssues(context.Background(), "assignee = currentUser()")
	if !IsClientError(err, ErrUnknown) {
		t.Errorf("expected unknown error, got %v", err)
	}
}

func TestSearchIssuesEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// no body
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	_, _, err := client.SearchIssues(context.Background(), "assignee = currentUser()")
	if !IsClientError(err, ErrUnknown) {
		t.Errorf("expected unknown error, got %v", err)
	}
}

func TestSearchIssuesNoIssuesReturned(t *testing.T) {
	payload := map[string]interface{}{
		"total":      0,
		"maxResults": 50,
		"issues":     []map[string]interface{}{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	issues, total, err := client.SearchIssues(context.Background(), "assignee = currentUser()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %d", total)
	}
	if issues == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestCurrentUserInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !IsClientError(err, ErrUnknown) {
		t.Errorf("expected unknown error, got %v", err)
	}
}

func TestFetchStatusesSuccess(t *testing.T) {
	payload := map[string]interface{}{
		"statuses": []map[string]interface{}{
			{"name": "To Do", "category": "New", "statusKey": "todo"},
			{"name": "In Progress", "category": "InProgress", "statusKey": "in-progress"},
			{"name": "Done", "category": "Done", "statusKey": "done"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		wantPath := "/rest/api/2/status"
		if r.URL.Path != wantPath {
			t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	statuses, err := client.FetchStatuses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
	if statuses[0].Name != "To Do" {
		t.Errorf("status 0 name = %q, want %q", statuses[0].Name, "To Do")
	}
	if statuses[0].StatusKey != "todo" {
		t.Errorf("status 0 statusKey = %q, want %q", statuses[0].StatusKey, "todo")
	}
	if statuses[1].Category != "InProgress" {
		t.Errorf("status 1 category = %q, want %q", statuses[1].Category, "InProgress")
	}
}

func TestFetchStatusesNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errorMessages":["Not found"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchStatuses(context.Background())
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestFetchStatusesAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchStatuses(context.Background())
	if !IsClientError(err, ErrAuth) {
		t.Errorf("expected auth error, got %v", err)
	}
}

func TestFetchStatusesInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchStatuses(context.Background())
	if !IsClientError(err, ErrUnknown) {
		t.Errorf("expected unknown error, got %v", err)
	}
}

func TestFetchSprintsSuccess(t *testing.T) {
	payload := map[string]interface{}{
		"values": []map[string]interface{}{
			{"id": 100, "name": "Sprint 1", "state": "active",
				"startDate": "2024-01-01T00:00:00Z",
				"endDate":   "2024-01-14T00:00:00Z"},
			{"id": 101, "name": "Sprint 2", "state": "closed"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		wantPath := "/rest/api/2/project/PROJ/versions"
		if r.URL.Path != wantPath {
			t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
		}
		if r.URL.Query().Get("project") != "PROJ" {
			t.Errorf("project query = %q, want PROJ", r.URL.Query().Get("project"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	sprints, err := client.FetchSprints(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprints) != 2 {
		t.Fatalf("expected 2 sprints, got %d", len(sprints))
	}
	if sprints[0].ID != 100 {
		t.Errorf("sprint 0 id = %d, want 100", sprints[0].ID)
	}
	if sprints[0].Name != "Sprint 1" {
		t.Errorf("sprint 0 name = %q, want %q", sprints[0].Name, "Sprint 1")
	}
	if sprints[0].State != "active" {
		t.Errorf("sprint 0 state = %q, want %q", sprints[0].State, "active")
	}
	if sprints[1].Name != "Sprint 2" {
		t.Errorf("sprint 1 name = %q, want %q", sprints[1].Name, "Sprint 2")
	}
}

func TestFetchSprintsEmptyProjectKey(t *testing.T) {
	client := NewJiraClient("https://jira.example.com")
	_, err := client.FetchSprints(context.Background(), "")
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestFetchSprintsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errorMessages":["Not found"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchSprints(context.Background(), "PROJ")
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestFetchSprintsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchSprints(context.Background(), "PROJ")
	if !IsClientError(err, ErrAuth) {
		t.Errorf("expected auth error, got %v", err)
	}
}

func TestFetchSprintsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchSprints(context.Background(), "PROJ")
	if !IsClientError(err, ErrUnknown) {
		t.Errorf("expected unknown error, got %v", err)
	}
}

func TestFetchFixVersionsSuccess(t *testing.T) {
	payload := map[string]interface{}{
		"values": []map[string]interface{}{
			{"name": "1.0.0", "description": "First release", "archived": false, "released": true},
			{"name": "1.1.0", "archived": false, "released": false},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		wantPath := "/rest/api/2/project/PROJ/versions"
		if r.URL.Path != wantPath {
			t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	versions, err := client.FetchFixVersions(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 fix versions, got %d", len(versions))
	}
	if versions[0].Name != "1.0.0" {
		t.Errorf("version 0 name = %q, want %q", versions[0].Name, "1.0.0")
	}
	if versions[0].Description != "First release" {
		t.Errorf("version 0 description = %q, want %q", versions[0].Description, "First release")
	}
	if versions[0].Released != true {
		t.Errorf("version 0 released = %v, want true", versions[0].Released)
	}
	if versions[1].Archived != false {
		t.Errorf("version 1 archived = %v, want false", versions[1].Archived)
	}
}

func TestFetchFixVersionsEmptyProjectKey(t *testing.T) {
	client := NewJiraClient("https://jira.example.com")
	_, err := client.FetchFixVersions(context.Background(), "")
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestFetchFixVersionsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errorMessages":["Not found"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchFixVersions(context.Background(), "PROJ")
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestFetchFixVersionsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchFixVersions(context.Background(), "PROJ")
	if !IsClientError(err, ErrAuth) {
		t.Errorf("expected auth error, got %v", err)
	}
}

func TestFetchFixVersionsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	_, err := client.FetchFixVersions(context.Background(), "PROJ")
	if !IsClientError(err, ErrUnknown) {
		t.Errorf("expected unknown error, got %v", err)
	}
}

func TestFetchStatusesTransportError(t *testing.T) {
	client := NewJiraClient("http://localhost:1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*1000000)
	defer cancel()
	_, err := client.FetchStatuses(ctx)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestFetchSprintsTransportError(t *testing.T) {
	client := NewJiraClient("http://localhost:1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*1000000)
	defer cancel()
	_, err := client.FetchSprints(ctx, "PROJ")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestFetchFixVersionsTransportError(t *testing.T) {
	client := NewJiraClient("http://localhost:1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*1000000)
	defer cancel()
	_, err := client.FetchFixVersions(ctx, "PROJ")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestUpdateIssueTransportError(t *testing.T) {
	client := NewJiraClient("http://localhost:1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*1000000)
	defer cancel()
	_, err := client.UpdateIssue(ctx, "PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42"},
	})
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestUpdateIssueEmptyKey(t *testing.T) {
	client := NewJiraClient("http://localhost:1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*1000000)
	defer cancel()
	_, err := client.UpdateIssue(ctx, "", &schema.Issue{})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestUpdateIssueSuccess(t *testing.T) {
	var gotPut, gotGet bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			gotPut = true
			if r.URL.Path != "/rest/api/2/issue/PROJ-42" || r.URL.RawQuery != "" {
				t.Errorf("PUT url = %s", r.URL.String())
			}
			var got map[string]map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode body: %v", err)
			}
			f := got["fields"]
			if f["summary"] != "Updated summary" || f["customfield_10002"] != float64(5) {
				t.Errorf("fields = %v", f)
			}
			for _, bad := range []string{"status", "customfield_sprint", "key", "reporter", "created", "updated", "resolution"} {
				if _, ok := f[bad]; ok {
					t.Errorf("read-only field %q sent", bad)
				}
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			gotGet = true
			if !gotPut {
				t.Error("refetch before PUT")
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"1","key":"PROJ-42","fields":{"summary":"Updated summary","issuetype":{"name":"Story"}}}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{"username": "user@example.com", "password": "secret"},
		},
	})

	issue := &schema.Issue{
		Identity:     schema.IssueIdentity{Key: "PROJ-42"},
		Summary:      "Updated summary",
		Status:       "Done",
		Sprint:       "Sprint 1",
		CustomFields: map[string]interface{}{"customfield_10002": 5},
	}
	updated, err := client.UpdateIssue(context.Background(), "PROJ-42", issue)
	if err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if !gotPut || !gotGet {
		t.Fatalf("put=%v get=%v", gotPut, gotGet)
	}
	if string(updated.Identity.Key) != "PROJ-42" || updated.Summary != "Updated summary" {
		t.Errorf("updated = %+v", updated)
	}
}

func TestUpdateIssueNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	_, err := client.UpdateIssue(context.Background(), "PROJ-99", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-99"},
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestBuildAuthenticatedRequestOAuth1(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		AuthType: AuthTypeOAuth1,
	}

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/2/issue/PROJ-1", nil)
	_, err := BuildAuthenticatedRequest(req, creds)
	if err == nil {
		t.Fatal("expected error for oauth1 auth type")
	}
	if !contains(err.Error(), "oauth1") {
		t.Errorf("error should mention oauth1, got %q", err.Error())
	}
	if !contains(err.Error(), "not yet implemented") {
		t.Errorf("error should mention not yet implemented, got %q", err.Error())
	}
}

func TestSearchIssuesNoCredentials(t *testing.T) {
	payload := map[string]interface{}{
		"total":      1,
		"maxResults": 50,
		"issues": []map[string]interface{}{
			{
				"id":  "10001",
				"key": "PROJ-10",
				"fields": map[string]interface{}{
					"issuetype": map[string]interface{}{
						"name": "Story",
					},
					"summary": "Issue without credentials",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	// Intentionally NOT calling SetCredentials.

	issues, total, err := client.SearchIssues(context.Background(), "assignee = currentUser()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if string(issues[0].Identity.Key) != "PROJ-10" {
		t.Errorf("issue 0 key = %q, want %q", issues[0].Identity.Key, "PROJ-10")
	}
}

func TestSearchIssuesForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errorMessages":"Access denied"}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	_, _, err := client.SearchIssues(context.Background(), "assignee = currentUser()")
	if !IsClientError(err, ErrAuth) {
		t.Errorf("expected auth error for 403, got %v", err)
	}
}

func TestSearchIssuesNoCredentialsAuthError(t *testing.T) {
	payload := map[string]interface{}{
		"total":      0,
		"maxResults": 50,
		"issues":     []map[string]interface{}{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	// Set credentials with missing required fields so
	// BuildAuthenticatedRequest returns an error.
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{},
		},
	})

	_, _, err := client.SearchIssues(context.Background(), "assignee = currentUser()")
	if !IsClientError(err, ErrUnknown) {
		t.Errorf("expected unknown error for auth construction failure, got %v", err)
	}
}

func TestUpdateIssueForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errorMessages":"Permission denied"}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	_, err := client.UpdateIssue(context.Background(), "PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42"},
	})
	if !IsClientError(err, ErrAuth) {
		t.Errorf("expected auth error for 403, got %v", err)
	}
}

func TestUpdateIssueUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errorMessages":"Unauthorized"}`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	_, err := client.UpdateIssue(context.Background(), "PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42"},
	})
	if !IsClientError(err, ErrAuth) {
		t.Errorf("expected auth error for 401, got %v", err)
	}
}

func TestUpdateIssueInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	_, err := client.UpdateIssue(context.Background(), "PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42"},
	})
	if !IsClientError(err, ErrUnknown) {
		t.Errorf("expected unknown error for invalid JSON, got %v", err)
	}
}

func TestUpdateIssueEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// no body
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{
				"username": "user@example.com",
				"password": "secret",
			},
		},
	})

	_, err := client.UpdateIssue(context.Background(), "PROJ-42", &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-42"},
	})
	if !IsClientError(err, ErrUnknown) {
		t.Errorf("expected unknown error for empty body, got %v", err)
	}
}

func TestFetchIssueRequestsExplicitFieldsAndNormalizesContext(t *testing.T) {
	fixture, err := os.ReadFile("../export/testdata/issue_with_context.json")
	if err != nil {
		t.Fatal(err)
	}
	var gotFields string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFields = r.URL.Query().Get("fields")
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
	client.epicLinkField = "customfield_10014"
	issue, err := client.FetchIssue(context.Background(), "SKOLELOGIN-18312")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"description", "status", "priority", "reporter", "created", "updated", "components", "parent", "fixVersions", "subtasks", "customfield_10014"} {
		if !strings.Contains(","+gotFields+",", ","+f+",") {
			t.Errorf("fields=%q missing %q", gotFields, f)
		}
	}
	if issue.Description == "" || issue.Context.Priority != "High" || issue.Context.EpicLink != "SKOLELOGIN-17000" {
		t.Errorf("unexpected issue: desc=%q ctx=%+v", issue.Description, issue.Context)
	}
}

func TestResolveEpicLinkField_EnvOverridesSetting(t *testing.T) {
	t.Setenv(EpicLinkFieldEnv, "")
	if got := ResolveEpicLinkField(" customfield_1 "); got != "customfield_1" {
		t.Errorf("setting only = %q", got)
	}
	if got := ResolveEpicLinkField(""); got != "" {
		t.Errorf("unset = %q", got)
	}
	t.Setenv(EpicLinkFieldEnv, "customfield_2")
	if got := ResolveEpicLinkField("customfield_1"); got != "customfield_2" {
		t.Errorf("env over setting = %q", got)
	}
}

func TestMapHTTPErrMessages(t *testing.T) {
	long := strings.Repeat("x", 500)
	tests := map[string]struct {
		status int
		body   string
		want   string
	}{
		"non-json html": {405, "<html>\n  <body>Method   Not Allowed</body>\n</html>", "jira: http: HTTP 405 Method Not Allowed: <html> <body>Method Not Allowed</body> </html>"},
		"empty body":    {502, "", "jira: http: HTTP 502 Bad Gateway"},
		"json no msg":   {500, `{}`, "jira: http: HTTP 500 Internal Server Error: {}"},
		"json errors":   {400, `{"errorMessages":["bad"],"errors":{"summary":"required"}}`, "jira: http: HTTP 400: bad; summary: required"},
		"long body":     {500, long, "jira: http: HTTP 500 Internal Server Error: " + long[:300] + "..."},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer server.Close()
			_, err := NewJiraClient(server.URL).FetchIssue(context.Background(), "PROJ-1")
			if err == nil || err.Error() != tc.want {
				t.Errorf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestFetchIssue_PopulatesRemoteStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"key":"P-1","fields":{"summary":"S","status":{"name":"Open"},"issuetype":{"name":"Task"}}}`))
	}))
	defer server.Close()
	got, err := NewJiraClient(server.URL).FetchIssue(context.Background(), "P-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "Open" {
		t.Errorf("Status = %q, want Open", got.Status)
	}
}

func TestProductionTransportRejectsUntrustedCert(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, WithTransport(productionTransport()))
	_, err := c.FetchIssue(context.Background(), "ABC-1")
	if err == nil {
		t.Fatal("expected certificate verification error, got nil")
	}
	if !strings.Contains(err.Error(), "x509") {
		t.Fatalf("expected x509 error, got %v", err)
	}
}

func TestClientTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	client := NewJiraClient(srv.URL, WithTimeout(50*time.Millisecond))
	start := time.Now()
	_, err := client.FetchIssue(context.Background(), "PROJ-1")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("FetchIssue took %v, want < 1s", elapsed)
	}
	var ce *ClientError
	if !errors.As(err, &ce) || ce.Code != ErrTransport {
		t.Fatalf("err = %#v, want transport ClientError", err)
	}
	host := strings.TrimPrefix(srv.URL, "http://")
	if !strings.Contains(err.Error(), "timed out after 50ms") || !strings.Contains(err.Error(), host) {
		t.Fatalf("err = %q, want timeout and host %q", err.Error(), host)
	}
}

func TestClientDefaultTimeout(t *testing.T) {
	if got := NewJiraClient("http://x").httpClient.Timeout; got != 30*time.Second {
		t.Fatalf("default timeout = %v, want 30s", got)
	}
	c := NewJiraClient("http://x", WithTimeout(time.Second), WithTransport(http.DefaultTransport))
	if c.httpClient.Timeout != time.Second {
		t.Fatalf("timeout after WithTransport = %v, want 1s", c.httpClient.Timeout)
	}
}

// stubRT answers each round trip from a scripted list of statuses and records
// the request bodies it saw.
type stubRT struct {
	statuses []int
	headers  map[string]string
	bodies   []string
	err      error
	calls    int
}

func (s *stubRT) RoundTrip(r *http.Request) (*http.Response, error) {
	s.calls++
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		s.bodies = append(s.bodies, string(b))
	}
	if s.err != nil && s.calls <= len(s.statuses) && s.statuses[s.calls-1] == 0 {
		return nil, s.err
	}
	i := s.calls - 1
	if i >= len(s.statuses) {
		i = len(s.statuses) - 1
	}
	h := http.Header{}
	if s.statuses[i] != 200 {
		for k, v := range s.headers {
			h.Set(k, v)
		}
	}
	body := `{"key":"A-1","fields":{}}`
	if r.URL.Path == "/rest/api/2/search" {
		body = `{"total":1,"issues":[{"key":"A-1","fields":{}}]}`
	}
	return &http.Response{StatusCode: s.statuses[i], Header: h, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

func retryClient(rt http.RoundTripper, slept *[]time.Duration) *JiraClient {
	return NewJiraClient("http://jira.test", WithTransport(rt), WithSleep(func(ctx context.Context, d time.Duration) error {
		*slept = append(*slept, d)
		return ctx.Err()
	}))
}

func TestRetryAfterThen200(t *testing.T) {
	rt := &stubRT{statuses: []int{429, 200}, headers: map[string]string{"Retry-After": "2"}}
	var slept []time.Duration
	if _, err := retryClient(rt, &slept).FetchIssue(context.Background(), "A-1"); err != nil {
		t.Fatal(err)
	}
	if len(slept) != 1 || slept[0] != 2*time.Second {
		t.Fatalf("slept = %v, want [2s]", slept)
	}
}

func TestRetryAfterCappedAndHTTPDate(t *testing.T) {
	var slept []time.Duration
	rt := &stubRT{statuses: []int{429, 200}, headers: map[string]string{"Retry-After": "3600"}}
	if _, err := retryClient(rt, &slept).FetchIssue(context.Background(), "A-1"); err != nil {
		t.Fatal(err)
	}
	if slept[0] != 60*time.Second {
		t.Fatalf("slept = %v, want 60s cap", slept)
	}
	slept = nil
	date := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	rt = &stubRT{statuses: []int{429, 200}, headers: map[string]string{"Retry-After": date}}
	if _, err := retryClient(rt, &slept).FetchIssue(context.Background(), "A-1"); err != nil {
		t.Fatal(err)
	}
	if slept[0] < 8*time.Second || slept[0] > 10*time.Second {
		t.Fatalf("slept = %v, want about 10s", slept)
	}
}

func TestRetryThree503ThenSuccess(t *testing.T) {
	rt := &stubRT{statuses: []int{503, 503, 503, 200}}
	var slept []time.Duration
	if _, err := retryClient(rt, &slept).FetchIssue(context.Background(), "A-1"); err != nil {
		t.Fatal(err)
	}
	if len(slept) != 3 {
		t.Fatalf("slept = %v, want 3 waits", slept)
	}
	for i, base := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second} {
		if slept[i] < base || slept[i] > base+base/4 {
			t.Errorf("wait %d = %v, want %v plus up to 25%% jitter", i, slept[i], base)
		}
	}
}

func TestRetryFour503Fails(t *testing.T) {
	rt := &stubRT{statuses: []int{503}}
	var slept []time.Duration
	_, err := retryClient(rt, &slept).FetchIssue(context.Background(), "A-1")
	if err == nil || !strings.Contains(err.Error(), "after 4 attempts") {
		t.Fatalf("err = %v, want 'after 4 attempts'", err)
	}
	if !IsClientError(err, ErrHTTP) || rt.calls != 4 {
		t.Fatalf("err = %v calls = %d, want http error after 4 calls", err, rt.calls)
	}
}

func TestRetryPostSearchReplaysBody(t *testing.T) {
	rt := &stubRT{statuses: []int{503, 200}}
	var slept []time.Duration
	if _, _, err := retryClient(rt, &slept).SearchIssues(context.Background(), "project = A"); err != nil {
		t.Fatal(err)
	}
	if len(rt.bodies) != 2 || rt.bodies[0] == "" || rt.bodies[0] != rt.bodies[1] {
		t.Fatalf("bodies = %q, want identical non-empty bodies", rt.bodies)
	}
}

func TestRetryNeverCreateOrTransition(t *testing.T) {
	issue := &schema.Issue{Identity: schema.IssueIdentity{Key: "A-1", Project: schema.TypedRef{Type: "project", Value: "A"}}}
	for name, call := range map[string]func(c *JiraClient) error{
		"CreateIssue": func(c *JiraClient) error { _, err := c.CreateIssue(context.Background(), issue); return err },
		"DoTransition": func(c *JiraClient) error {
			return c.DoTransition(context.Background(), "A-1", "11", nil)
		},
	} {
		rt := &stubRT{statuses: []int{429}}
		var slept []time.Duration
		err := call(retryClient(rt, &slept))
		if err == nil || rt.calls != 1 || len(slept) != 0 {
			t.Errorf("%s: err=%v calls=%d slept=%v, want one call and no retry", name, err, rt.calls, slept)
		}
	}
}

func TestRetryConnectionResetGetOnly(t *testing.T) {
	reset := errors.New("read tcp: connection reset by peer")
	rt := &stubRT{statuses: []int{0, 200}, err: reset}
	var slept []time.Duration
	if _, err := retryClient(rt, &slept).FetchIssue(context.Background(), "A-1"); err != nil || rt.calls != 2 {
		t.Fatalf("GET: err=%v calls=%d, want success on retry", err, rt.calls)
	}
	rt = &stubRT{statuses: []int{0, 200}, err: reset}
	if _, _, err := retryClient(rt, &slept).SearchIssues(context.Background(), "project = A"); err == nil || rt.calls != 1 {
		t.Fatalf("POST search: err=%v calls=%d, want no retry on reset", err, rt.calls)
	}
}

func TestRetrySleepHonoursContext(t *testing.T) {
	rt := &stubRT{statuses: []int{503}}
	ctx, cancel := context.WithCancel(context.Background())
	c := NewJiraClient("http://jira.test", WithTransport(rt), WithSleep(func(ctx context.Context, d time.Duration) error {
		cancel()
		<-ctx.Done()
		return ctx.Err()
	}))
	start := time.Now()
	_, err := c.FetchIssue(ctx, "A-1")
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("err = %v, want context canceled", err)
	}
	if rt.calls != 1 || time.Since(start) > time.Second {
		t.Fatalf("calls=%d elapsed=%v, want prompt return", rt.calls, time.Since(start))
	}
}

func TestRealSleepReturnsOnCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := realSleep(ctx, time.Minute); err == nil || time.Since(start) > time.Second {
		t.Fatalf("realSleep err=%v elapsed=%v, want prompt cancellation", err, time.Since(start))
	}
}

type searchLogEntry struct{ method, path, token string }

func searchTestClient(url string) *JiraClient {
	c := NewJiraClient(url)
	c.SetCredentials(config.ResolvedInstanceCredentials{
		AuthType: "basic",
		Credential: config.ResolvedCredential{
			Fields: map[string]string{"username": "u@example.com", "password": "secret"},
		},
	})
	return c
}

func TestSearchIssuesChainsNextPageTokens(t *testing.T) {
	var log []searchLogEntry
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		tok, _ := body["nextPageToken"].(string)
		log = append(log, searchLogEntry{r.Method, r.URL.Path, tok})
		if _, ok := body["startAt"]; ok {
			t.Errorf("startAt sent to search/jql")
		}
		switch tok {
		case "":
			w.Write([]byte(`{"issues":[{"key":"P-1"},{"key":"P-2"}],"nextPageToken":"tokA"}`))
		case "tokA":
			w.Write([]byte(`{"issues":[{"key":"P-3"}],"nextPageToken":"tokB"}`))
		case "tokB":
			w.Write([]byte(`{"issues":[{"key":"P-4"}],"isLast":true}`))
		default:
			t.Errorf("unexpected token %q", tok)
		}
	}))
	defer srv.Close()

	issues, total, err := searchTestClient(srv.URL).SearchIssues(context.Background(), "project = P")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, i := range issues {
		keys = append(keys, string(i.Identity.Key))
	}
	if strings.Join(keys, ",") != "P-1,P-2,P-3,P-4" || total != 4 {
		t.Errorf("keys = %v total = %d", keys, total)
	}
	for i, e := range log {
		t.Logf("request %d: %s %s token=%q", i+1, e.method, e.path, e.token)
	}
	want := []searchLogEntry{
		{"POST", "/rest/api/3/search/jql", ""},
		{"POST", "/rest/api/3/search/jql", "tokA"},
		{"POST", "/rest/api/3/search/jql", "tokB"},
	}
	if len(log) != len(want) {
		t.Fatalf("log = %v", log)
	}
	for i := range want {
		if log[i] != want[i] {
			t.Errorf("log[%d] = %v, want %v", i, log[i], want[i])
		}
	}
}

func TestSearchIssuesFallsBackOnceToLegacy(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusGone} {
		var paths []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			if r.URL.Path == "/rest/api/3/search/jql" {
				w.WriteHeader(status)
				return
			}
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["startAt"] == nil {
				t.Errorf("startAt missing on legacy path")
			}
			w.Write([]byte(`{"total":1,"issues":[{"key":"P-9"}]}`))
		}))
		c := searchTestClient(srv.URL)
		for call := 0; call < 2; call++ {
			issues, total, err := c.SearchIssues(context.Background(), "project = P")
			if err != nil || len(issues) != 1 || total != 1 {
				t.Fatalf("status %d call %d: %v %d %v", status, call, issues, total, err)
			}
		}
		srv.Close()
		want := "/rest/api/3/search/jql,/rest/api/2/search,/rest/api/2/search"
		if strings.Join(paths, ",") != want {
			t.Errorf("status %d paths = %v, want %s", status, paths, want)
		}
	}
}

func TestSearchIssuesCustomFieldsBothPaths(t *testing.T) {
	const resp = `{"total":1,"issues":[{"key":"P-1","fields":{"customfield_12601":"x"}}],"names":{"customfield_12601":"Accept"},"isLast":true}`
	for _, legacy := range []bool{false, true} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if legacy && r.URL.Path == "/rest/api/3/search/jql" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write([]byte(resp))
		}))
		c := searchTestClient(srv.URL)
		c.customFields = []string{"customfield_12601"}
		issues, _, err := c.SearchIssues(context.Background(), "project = P")
		srv.Close()
		if err != nil || len(issues) != 1 {
			t.Fatalf("legacy=%v: %v", legacy, err)
		}
		if issues[0].CustomFields["customfield_12601"] != "x" || issues[0].Context.CustomFieldNames["customfield_12601"] != "Accept" {
			t.Errorf("legacy=%v: %+v", legacy, issues[0])
		}
	}
}

func TestSearchIssuesNotFoundOnBothEndpoints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	_, _, err := searchTestClient(srv.URL).SearchIssues(context.Background(), "project = P")
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestRetryIgnoresServerSentAttemptsHeader(t *testing.T) {
	rt := &stubRT{statuses: []int{503}, headers: map[string]string{"X-Jirafs-Attempts": "99"}}
	var slept []time.Duration
	_, err := retryClient(rt, &slept).FetchIssue(context.Background(), "A-1")
	if err == nil || strings.Contains(err.Error(), "99") || !strings.Contains(err.Error(), "after 4 attempts") {
		t.Fatalf("err = %v, want 4 attempts and no 99", err)
	}
	rt = &stubRT{statuses: []int{429}, headers: map[string]string{"X-Jirafs-Attempts": "99"}}
	c := NewJiraClient("http://jira.test", WithTransport(rt), WithSleep(func(context.Context, time.Duration) error { return nil }))
	req, _ := http.NewRequest(http.MethodPost, "http://jira.test/rest/api/2/issue", nil)
	resp, err := c.do(req)
	if err != nil {
		t.Fatal(err)
	}
	if e := mapHTTPErr(resp); strings.Contains(e.Message, "attempts") {
		t.Fatalf("message = %q, header must not matter", e.Message)
	}
}

func TestRetryCancelDuringWaitIsContextCanceled(t *testing.T) {
	rt := &stubRT{statuses: []int{503}}
	ctx, cancel := context.WithCancel(context.Background())
	c := NewJiraClient("http://jira.test", WithTransport(rt), WithSleep(func(ctx context.Context, d time.Duration) error {
		cancel()
		return ctx.Err()
	}))
	_, err := c.FetchIssue(ctx, "A-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if !strings.Contains(err.Error(), "request failed: context canceled") || !IsClientError(err, ErrTransport) {
		t.Fatalf("err = %v, message or code changed", err)
	}
}
