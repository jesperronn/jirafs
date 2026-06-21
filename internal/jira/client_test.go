package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
