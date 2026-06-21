package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
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
		"id":   "10001",
		"key":  "PROJ-123",
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
		wantPath := "/rest/api/3/issue/PROJ-123"
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
	client := NewJiraClient("http://localhost:1") // unlikely to be listening
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
		"id":   "10002",
		"key":  "PROJ-456",
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
		"id":   "10003",
		"key":  "PROJ-789",
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
		"id":   "10004",
		"key":  "PROJ-000",
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
	if err.Error() != "jira: http: HTTP error" {
		t.Errorf("error = %q, want %q", err.Error(), "jira: http: HTTP error")
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
	if err.Error() != "jira: http: HTTP error" {
		t.Errorf("error = %q, want %q", err.Error(), "jira: http: HTTP error")
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
		BaseURL: "https://jira.example.com",
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

	req, err := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/3/issue/PROJ-1", nil)
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
		BaseURL: "https://jira.example.com",
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

	req, err := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/3/issue/PROJ-1", nil)
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

func TestBuildAuthenticatedRequestEmptyAuthType(t *testing.T) {
	creds := config.ResolvedInstanceCredentials{
		BaseURL:    "https://jira.example.com",
		AuthType:   "",
		Credential: config.ResolvedCredential{},
	}

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/3/issue/PROJ-1", nil)
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

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/3/issue/PROJ-1", nil)
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

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/3/issue/PROJ-1", nil)
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

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/3/issue/PROJ-1", nil)
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

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/3/issue/PROJ-1", nil)
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

	req, _ := http.NewRequest(http.MethodGet, "https://jira.example.com/rest/api/3/issue/PROJ-1", nil)
	_, err := BuildAuthenticatedRequest(req, creds)
	if err == nil {
		t.Fatal("expected error for missing api_token")
	}
	if !contains(err.Error(), "api_token") {
		t.Errorf("error should mention api_token, got %q", err.Error())
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
		"self":         "https://jira.example.com/rest/api/3/user?username=john.doe",
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
		wantPath := "/rest/api/3/myself"
		if r.URL.Path != wantPath {
			t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL)
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
		"total":    2,
		"maxResults": 50,
		"issues": []map[string]interface{}{
			{
				"id":   "10001",
				"key":  "PROJ-10",
				"fields": map[string]interface{}{
					"issuetype": map[string]interface{}{
						"name": "Story",
					},
					"summary": "First issue",
					"labels":  []string{"urgent"},
				},
			},
			{
				"id":   "10002",
				"key":  "PROJ-11",
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
		wantPath := "/rest/api/3/search"
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

	issues, err := client.SearchIssues(context.Background(), "my-issues")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestSearchIssuesUnknownScope(t *testing.T) {
	client := NewJiraClient("https://jira.example.com")
	_, err := client.SearchIssues(context.Background(), "unknown-scope")
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

	_, err := client.SearchIssues(context.Background(), "my-issues")
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
	_, err := client.SearchIssues(context.Background(), "my-issues")
	if !IsClientError(err, ErrHTTP) {
		t.Errorf("expected http error, got %v", err)
	}
}

func TestSearchIssuesTransportError(t *testing.T) {
	client := NewJiraClient("http://localhost:1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*1000000)
	defer cancel()
	_, err := client.SearchIssues(ctx, "my-issues")
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

	_, err := client.SearchIssues(context.Background(), "my-issues")
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

	_, err := client.SearchIssues(context.Background(), "my-issues")
	if !IsClientError(err, ErrUnknown) {
		t.Errorf("expected unknown error, got %v", err)
	}
}

func TestSearchIssuesNoIssuesReturned(t *testing.T) {
	payload := map[string]interface{}{
		"total":    0,
		"maxResults": 50,
		"issues":   []map[string]interface{}{},
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

	issues, err := client.SearchIssues(context.Background(), "my-issues")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
