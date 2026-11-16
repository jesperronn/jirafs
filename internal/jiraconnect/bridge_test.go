package jiraconnect

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/merge"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/plan"
	consumersync "github.com/jirafs/jirafs/internal/sync"

	"github.com/jirafs/jirafs/internal/config"
	legacy "github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func configured(t *testing.T, rt transport) *Client {
	t.Helper()
	c := New(Config{BaseURL: "https://example.invalid/jira", Transport: rt, EpicLinkField: "customfield_10", CustomFields: []string{"customfield_11", "customfield_12"}})
	c.SetCredentials(config.ResolvedInstanceCredentials{AuthType: "bearer_token", Credential: config.ResolvedCredential{Fields: map[string]string{"bearer_token": "offline-test"}}})
	return c
}

const issueJSON = `{"id":"1","key":"TEST-1","names":{"customfield_11":"Large number"},"fields":{"summary":"one","description":null,"labels":[],"assignee":null,"issuetype":{"name":"Story"},"project":{"key":"TEST"},"status":{"name":"Open"},"customfield_10":"TEST-9","customfield_11":9007199254740993,"customfield_12":{"id":"3","value":"Choice"},"issuelinks":[{"id":"5","type":{"name":"Blocks","inward":"is blocked by","outward":"blocks"},"inwardIssue":{"key":"TEST-2","fields":{"summary":"linked","status":{"name":"Done"}}}}],"subtasks":[{"key":"TEST-3","fields":{"summary":"child","status":{"name":"Open"}}}]}}`

func TestFetchPreservesConsumerSchemaAndExactCustomInteger(t *testing.T) {
	c := configured(t, func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer offline-test" {
			t.Fatal("missing credentials")
		}
		if !strings.Contains(r.URL.Query().Get("fields"), "customfield_10") {
			t.Fatal("missing epic field")
		}
		return response(200, issueJSON), nil
	})
	issue, e := c.FetchIssue(context.Background(), "TEST-1")
	if e != nil {
		t.Fatal(e)
	}
	if issue.Identity.Project.Value != "TEST" || issue.Context.EpicLink != "TEST-9" || issue.Description != "" || issue.Assignee != nil {
		t.Fatalf("bad mapping: %#v", issue)
	}
	if issue.CustomFields["customfield_11"] != int64(9007199254740993) || issue.Context.CustomFieldNames["customfield_11"] != "Large number" {
		t.Fatalf("lost custom value: %#v", issue.CustomFields)
	}
	if len(issue.LinkedIssues) != 1 || issue.LinkedIssues[0].Direction != "inward" || issue.LinkedIssues[0].Summary != "linked" || issue.LinkedIssues[0].Status != "Done" || len(issue.Subtasks) != 1 {
		t.Fatalf("lost context: %#v", issue)
	}
}
func TestSearchExhaustsCursor(t *testing.T) {
	calls := 0
	c := configured(t, func(r *http.Request) (*http.Response, error) {
		calls++
		var input struct{ StartAt int }
		json.NewDecoder(r.Body).Decode(&input)
		if input.StartAt != calls-1 {
			t.Fatalf("offset %d", input.StartAt)
		}
		entry := strings.ReplaceAll(issueJSON, `"id":"1"`, `"id":"`+string(rune('0'+calls))+`"`)
		entry = strings.ReplaceAll(entry, "TEST-1", "TEST-"+string(rune('0'+calls)))
		return response(200, `{"startAt":`+string(rune('0'+calls-1))+`,"maxResults":100,"total":2,"issues":[`+entry+`]}`), nil
	})
	issues, total, e := c.SearchIssues(context.Background(), "project = TEST ORDER BY key")
	if e != nil || len(issues) != 2 || total != 2 || calls != 2 {
		t.Fatalf("%d %d %d %v", len(issues), total, calls, e)
	}
}
func TestUpdateClearsAndRefreshFailureIsAcceptedWrite(t *testing.T) {
	puts := 0
	c := configured(t, func(r *http.Request) (*http.Response, error) {
		if r.Method == "PUT" {
			puts++
			var p struct{ Fields map[string]json.RawMessage }
			json.NewDecoder(r.Body).Decode(&p)
			if string(p.Fields["labels"]) != "[]" || string(p.Fields["fixVersions"]) != "[]" || string(p.Fields["customfield_11"]) != "null" {
				t.Fatalf("lost clears: %s", p.Fields)
			}
			if _, ok := p.Fields["status"]; ok {
				t.Fatal("sent status")
			}
			if _, ok := p.Fields["customfield_sprint"]; ok {
				t.Fatal("sent sprint placeholder")
			}
			if _, ok := p.Fields["priority"]; ok {
				t.Fatal("sent unchanged priority")
			}
			if len(p.Fields["issuelinks"]) == 0 {
				t.Fatal("changed existing link PUT behavior")
			}
			return response(204, ""), nil
		}
		return response(404, `{}`), nil
	})
	_, e := c.UpdateIssue(context.Background(), "TEST-1", &schema.Issue{Summary: "changed", Status: "Done", Sprint: "named", Labels: []string{}, FixVersions: []string{}, ClearFields: []string{"labels", "fixVersions"}, Context: schema.IssueContext{Priority: "Low"}, CustomFields: map[string]interface{}{"customfield_11": nil}, LinkedIssues: []schema.LinkedIssue{{Key: "TEST-2", LinkType: "Blocks", Direction: "outward"}}})
	var partial *RefreshAfterWriteError
	if puts != 1 || !errors.As(e, &partial) || !legacy.IsClientError(e, legacy.ErrNotFound) || !strings.Contains(e.Error(), "accepted update") {
		t.Fatalf("%d %v", puts, e)
	}
}
func TestConsumerErrorAndCredentialReplacement(t *testing.T) {
	c := configured(t, func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Basic dTpw" {
			t.Fatalf("old credentials: %s", r.Header.Get("Authorization"))
		}
		return response(403, `{}`), nil
	})
	c.SetCredentials(config.ResolvedInstanceCredentials{AuthType: "basic", Credential: config.ResolvedCredential{Fields: map[string]string{"username": "u", "password": "p"}}})
	_, e := c.FetchIssue(context.Background(), "TEST-1")
	if !legacy.IsClientError(e, legacy.ErrAuth) {
		t.Fatal(e)
	}
	c.SetCredentials(config.ResolvedInstanceCredentials{AuthType: "oauth1"})
	_, e = c.FetchIssue(context.Background(), "TEST-1")
	if !legacy.IsClientError(e, legacy.ErrAuth) {
		t.Fatal(e)
	}
}

func TestRemainingClientMethodsTranslateWireShapes(t *testing.T) {
	seen := map[string]bool{}
	c := configured(t, func(r *http.Request) (*http.Response, error) {
		path := strings.TrimPrefix(r.URL.Path, "/jira")
		seen[r.Method+" "+path] = true
		switch path {
		case "/rest/api/2/myself":
			return response(200, `{"name":"operator","displayName":"Operator","timeZone":"UTC"}`), nil
		case "/rest/api/2/status":
			return response(200, `[{"id":"1","name":"Open","statusCategory":{"key":"new"}}]`), nil
		case "/rest/api/2/project/TEST/versions":
			return response(200, `[{"id":"1","name":"v1","released":true,"archived":false}]`), nil
		case "/rest/agile/1.0/board":
			return response(200, `{"startAt":0,"maxResults":100,"total":1,"isLast":true,"values":[{"id":1,"type":"scrum"}]}`), nil
		case "/rest/agile/1.0/board/1/sprint":
			return response(200, `{"startAt":0,"maxResults":100,"total":1,"isLast":true,"values":[{"id":4,"name":"Sprint 4","state":"active"}]}`), nil
		case "/rest/api/2/issue/TEST-1/transitions":
			if r.Method == "POST" {
				var payload struct {
					Transition struct{ ID string }
					Fields     map[string]json.RawMessage
				}
				if e := json.NewDecoder(r.Body).Decode(&payload); e != nil || payload.Transition.ID != "7" || string(payload.Fields["resolution"]) != "null" {
					t.Fatalf("bad transition: %#v %v", payload, e)
				}
				return response(204, ""), nil
			}
			return response(200, `{"transitions":[{"id":"7","name":"Close","to":{"name":"Done"},"fields":{"resolution":{"required":true,"name":"Resolution"}}}]}`), nil
		case "/rest/api/2/issue/TEST-1/comment":
			return response(200, `{"startAt":0,"maxResults":2,"total":2,"comments":[{"id":"1","author":{"name":"old"},"created":"2026-01-01T00:00:00.000+0000","body":"old"},{"id":"2","author":{"displayName":"New"},"created":"2026-01-02T00:00:00Z","body":"new"}]}`), nil
		case "/rest/api/2/issue/createmeta/TEST/issuetypes":
			return response(200, `{"startAt":0,"maxResults":100,"total":1,"values":[{"id":"8","name":"Story"}]}`), nil
		case "/rest/api/2/issue/createmeta/TEST/issuetypes/8":
			return response(200, `{"startAt":0,"maxResults":100,"total":1,"values":[{"fieldId":"summary","name":"Summary","required":true,"hasDefaultValue":false,"allowedValues":[{"id":"x","value":"example"}]}]}`), nil
		case "/rest/api/2/issue":
			var payload struct{ Fields map[string]json.RawMessage }
			json.NewDecoder(r.Body).Decode(&payload)
			if string(payload.Fields["project"]) != `{"key":"TEST"}` || string(payload.Fields["issuetype"]) != `{"name":"Story"}` {
				t.Fatalf("bad create fields: %s", payload.Fields)
			}
			if _, ok := payload.Fields["issuelinks"]; ok {
				t.Fatal("create unexpectedly sends links")
			}
			return response(201, `{"id":"99","key":"TEST-99"}`), nil
		default:
			t.Fatalf("unexpected route %s", path)
			return nil, nil
		}
	})
	ctx := context.Background()
	user, e := c.CurrentUser(ctx)
	if e != nil || user.Name != "operator" {
		t.Fatalf("user: %v %v", user, e)
	}
	statuses, e := c.FetchStatuses(ctx)
	if e != nil || len(statuses) != 1 || statuses[0].Category != "new" || statuses[0].StatusKey != "" {
		t.Fatalf("statuses: %v %v", statuses, e)
	}
	versions, e := c.FetchFixVersions(ctx, "TEST")
	if e != nil || len(versions) != 1 || !versions[0].Released {
		t.Fatalf("versions: %v %v", versions, e)
	}
	sprints, e := c.FetchSprints(ctx, "TEST")
	if e != nil || len(sprints) != 1 || sprints[0].ID != 4 {
		t.Fatalf("sprints: %v %v", sprints, e)
	}
	transitions, e := c.FetchTransitions(ctx, "TEST-1")
	if e != nil || len(transitions) != 1 || len(transitions[0].RequiredFields()) != 1 {
		t.Fatalf("transitions: %v %v", transitions, e)
	}
	if e = c.DoTransition(ctx, "TEST-1", "7", map[string]interface{}{"resolution": nil}); e != nil {
		t.Fatal(e)
	}
	comments, total, e := c.FetchComments(ctx, "TEST-1", 2)
	if e != nil || total != 2 || len(comments) != 2 || comments[0].Author != "New" || comments[1].Created != "2026-01-01T00:00:00Z" {
		t.Fatalf("comments: %v %d %v", comments, total, e)
	}
	meta, e := c.FetchCreateMeta(ctx, "TEST", "story")
	if e != nil || len(meta) != 1 || meta[0].ID != "summary" || !meta[0].Required || meta[0].AllowedValues[0].Label() != "example" {
		t.Fatalf("metadata: %v %v", meta, e)
	}
	key, e := c.CreateIssue(ctx, &schema.Issue{Identity: schema.IssueIdentity{Project: schema.TypedRef{Type: schema.RefProject, Value: "TEST"}, Type: "story"}, Summary: "draft", LinkedIssues: []schema.LinkedIssue{{Key: "TEST-1", LinkType: "Blocks"}}})
	if e != nil || key != "TEST-99" {
		t.Fatalf("create: %s %v", key, e)
	}
	if len(seen) != 11 {
		t.Fatalf("unexpected route coverage: %v", seen)
	}
}

func TestCustomDecimalSurvivesConsumerYAMLRoundTrip(t *testing.T) {
	for _, number := range []string{"1.5", "0.1", "1.2300"} {
		c := configured(t, func(*http.Request) (*http.Response, error) {
			return response(200, strings.Replace(issueJSON, "9007199254740993", number, 1)), nil
		})
		issue, err := c.FetchIssue(context.Background(), "TEST-1")
		if err != nil {
			t.Fatal(err)
		}
		parsed, parseErr := schema.ParseIssue(schema.RenderIssue(*issue))
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if _, stringified := parsed.CustomFields["customfield_11"].(string); stringified {
			t.Fatalf("number became text: %#v", parsed.CustomFields)
		}
		original, _ := json.Marshal(issue.CustomFields["customfield_11"])
		roundtrip, _ := json.Marshal(parsed.CustomFields["customfield_11"])
		if string(original) != string(roundtrip) {
			t.Fatalf("number changed: %s -> %s", original, roundtrip)
		}
	}
	c := configured(t, func(*http.Request) (*http.Response, error) {
		return response(200, strings.Replace(issueJSON, "9007199254740993", "1.000000000000000000000000001", 1)), nil
	})
	if _, err := c.FetchIssue(context.Background(), "TEST-1"); err == nil {
		t.Fatal("silently accepted consumer-incompatible number")
	}
}
func TestReadTransportClassificationPreservesCause(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded, errors.New("private network details")} {
		c := configured(t, func(*http.Request) (*http.Response, error) { return nil, cause })
		_, err := c.FetchIssue(context.Background(), "TEST-1")
		if !legacy.IsClientError(err, legacy.ErrTransport) || strings.Contains(err.Error(), "private") {
			t.Fatalf("wrong safe transport error: %v", err)
		}
		if cause == context.DeadlineExceeded && !errors.Is(err, cause) {
			t.Fatal("lost deadline cause")
		}
	}
}

// Exercise the real consumer algorithms with adapter-produced values, rather
// than duplicating conflict/hash logic inside the adapter.
func TestConsumerBaseHashAndConcurrentEditRefuseWrite(t *testing.T) {
	ctx := context.Background()
	current := strings.Replace(issueJSON, `"summary":"one"`, `"updated":"2026-01-01T00:00:00Z","summary":"one"`, 1)
	puts := 0
	c := configured(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" {
			puts++
			t.Fatalf("conflict attempted remote write: %s", r.Method)
		}
		return response(200, current), nil
	})
	base, err := c.FetchIssue(ctx, "TEST-1")
	if err != nil {
		t.Fatal(err)
	}
	baseDoc := schema.RenderIssue(*base)
	stamped := schema.StampSyncedHash(baseDoc)
	if schema.SyncedHash(stamped) != schema.ContentHash(baseDoc) || schema.ContentHash(stamped) != schema.ContentHash(baseDoc) {
		t.Fatal("consumer remote base hash changed while stamping")
	}
	mirrorDir := t.TempDir()
	if err = mirror.SaveBase(mirrorDir, base); err != nil {
		t.Fatal(err)
	}
	stored, ok := mirror.LoadBase(mirrorDir, "TEST-1")
	if !ok || schema.ContentHash(schema.RenderIssue(*stored)) != schema.ContentHash(baseDoc) {
		t.Fatal("consumer stored base did not preserve adapter remote state")
	}
	editedDoc := strings.Replace(stamped, "summary: 'one'", "summary: 'local edit'", 1)
	if editedDoc == stamped {
		t.Fatalf("consumer summary rendering changed: %s", stamped)
	}
	if schema.ContentHash(editedDoc) == schema.SyncedHash(editedDoc) {
		t.Fatal("user edit not detected by consumer hash")
	}
	local, parseErr := schema.ParseIssue(editedDoc)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	current = strings.ReplaceAll(strings.Replace(current, `"summary":"one"`, `"summary":"concurrent remote edit"`, 1), "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z")
	remote, err := c.FetchIssue(ctx, "TEST-1")
	if err != nil {
		t.Fatal(err)
	}
	merged := merge.Issue(stored, local, *remote)
	if len(merged.Conflicts) != 1 || merged.Conflicts[0].Field != schema.EditableFieldSummary || merged.Conflicts[0].Base != "one" || merged.Conflicts[0].Remote != "concurrent remote edit" || merged.Merged.Summary != "local edit" || merged.NewBase.Summary != "one" {
		t.Fatalf("consumer conflict record lost either side/base: %#v", merged)
	}
	ops, _, err := plan.BuildPlanWithBase(stored, local, *remote)
	if err != nil {
		t.Fatal(err)
	}
	result := consumersync.Sync(stored, local, *remote, ops, t.TempDir())
	if len(result.Conflicts) != 1 || result.Conflicts[0].Field != schema.EditableFieldSummary || result.Conflicts[0].Type != schema.ConflictBothEdited || result.Remote.Summary != "concurrent remote edit" {
		t.Fatalf("consumer sync failed to refuse concurrent edit: %#v", result)
	}
	// This is the consumer orchestration gate: only validated plans reach HTTP.
	if len(result.Conflicts) == 0 {
		if _, err = c.UpdateIssue(ctx, "TEST-1", result.Remote); err != nil {
			t.Fatal(err)
		}
	}
	if puts != 0 {
		t.Fatalf("conflicted plan produced %d writes", puts)
	}
}

func TestConsumerCleanPlanSyncClearsCustomFieldsLinksAndRefresh(t *testing.T) {
	ctx := context.Background()
	baseWire := strings.Replace(issueJSON, `"labels":[]`, `"labels":["old"],"fixVersions":[{"id":"8","name":"v1"}]`, 1)
	baseWire = strings.Replace(baseWire, `"summary":"one"`, `"updated":"2026-01-01T00:00:00Z","summary":"one"`, 1)
	afterWire := strings.ReplaceAll(strings.Replace(baseWire, `"summary":"one"`, `"summary":"local edit"`, 1), "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z")
	afterWire = strings.Replace(afterWire, `"labels":["old"]`, `"labels":[]`, 1)
	afterWire = strings.Replace(afterWire, `"fixVersions":[{"id":"8","name":"v1"}]`, `"fixVersions":[]`, 1)
	afterWire = strings.Replace(afterWire, `"customfield_11":9007199254740993`, `"customfield_11":42`, 1)
	afterWire = strings.Replace(afterWire, `"customfield_12":{"id":"3","value":"Choice"}`, `"customfield_12":null`, 1)
	gets, puts := 0, 0
	c := configured(t, func(r *http.Request) (*http.Response, error) {
		if r.Method == "PUT" {
			puts++
			var body struct{ Fields map[string]json.RawMessage }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			for field, want := range map[string]string{"summary": `"local edit"`, "labels": "[]", "fixVersions": "[]", "customfield_11": "42", "customfield_12": "null"} {
				if string(body.Fields[field]) != want {
					t.Fatalf("consumer planned %s lost in adapter PUT: %s", field, body.Fields[field])
				}
			}
			var links []struct {
				Type        struct{ Name string }
				InwardIssue struct{ Key string }
			}
			if err := json.Unmarshal(body.Fields["issuelinks"], &links); err != nil || len(links) != 1 || links[0].Type.Name != "Blocks" || links[0].InwardIssue.Key != "TEST-2" {
				t.Fatalf("consumer links changed: %s (%v)", body.Fields["issuelinks"], err)
			}
			if _, ok := body.Fields["status"]; ok {
				t.Fatal("consumer status leaked into field PUT")
			}
			return response(204, ""), nil
		}
		if r.Method != "GET" {
			t.Fatalf("unexpected method %s", r.Method)
		}
		gets++
		if puts > 0 {
			return response(200, afterWire), nil
		}
		return response(200, baseWire), nil
	})
	base, err := c.FetchIssue(ctx, "TEST-1")
	if err != nil {
		t.Fatal(err)
	}
	local, parseErr := schema.ParseIssue(schema.RenderIssue(*base))
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	local.Summary = "local edit"
	local.Labels = nil
	local.FixVersions = nil
	local.CustomFields["customfield_11"] = int64(42)
	local.CustomFields["customfield_12"] = nil
	remote, err := c.FetchIssue(ctx, "TEST-1")
	if err != nil {
		t.Fatal(err)
	}
	merged := merge.Issue(base, local, *remote)
	if len(merged.Conflicts) != 0 || merged.Merged.Summary != "local edit" || len(merged.Merged.LinkedIssues) != 1 {
		t.Fatalf("clean consumer merge failed: %#v", merged)
	}
	ops, conflicts, err := plan.BuildPlanWithBase(base, local, *remote)
	if err != nil || len(conflicts) != 0 || len(ops) != 4 {
		t.Fatalf("consumer plan = %#v; conflicts=%#v; err=%v", ops, conflicts, err)
	}
	result := consumersync.Sync(base, local, *remote, ops, t.TempDir())
	if len(result.Conflicts) != 0 || result.Remote == nil {
		t.Fatalf("consumer rejected clean plan: %#v", result)
	}
	clear := map[string]bool{}
	for _, field := range result.Remote.ClearFields {
		clear[field] = true
	}
	if !clear["labels"] || !clear["fixVersions"] {
		t.Fatalf("consumer omitted clear markers: %#v", result.Remote.ClearFields)
	}
	refreshed, err := c.UpdateIssue(ctx, "TEST-1", result.Remote)
	if err != nil {
		t.Fatal(err)
	}
	if puts != 1 || gets != 3 || refreshed.Summary != "local edit" || refreshed.Context.Updated != "2026-01-02T00:00:00Z" || len(refreshed.Labels) != 0 || len(refreshed.FixVersions) != 0 || refreshed.CustomFields["customfield_11"] != int64(42) || len(refreshed.LinkedIssues) != 1 {
		t.Fatalf("consumer update/refresh contract: puts=%d gets=%d issue=%#v", puts, gets, refreshed)
	}
	syncedDoc := schema.StampSyncedHash(schema.RenderIssue(*refreshed))
	if schema.SyncedHash(syncedDoc) != schema.ContentHash(syncedDoc) {
		t.Fatal("refreshed remote cannot be marked clean by consumer hash")
	}
}

func TestFullSearchRejectsNonAdjacentDuplicateAndAmbiguousType(t *testing.T) {
	calls := 0
	c := configured(t, func(*http.Request) (*http.Response, error) {
		calls++
		id := "1"
		if calls == 2 {
			id = "2"
		}
		entry := strings.Replace(issueJSON, `"id":"1"`, `"id":"`+id+`"`, 1)
		entry = strings.ReplaceAll(entry, "TEST-1", "TEST-"+id)
		return response(200, `{"startAt":`+string(rune('0'+calls-1))+`,"maxResults":1,"total":3,"issues":[`+entry+`]}`), nil
	})
	if issues, _, err := c.SearchIssues(context.Background(), "project=TEST ORDER BY key"); err == nil || issues != nil {
		t.Fatal("accepted duplicate issue across full search")
	}
	c = configured(t, func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, "/issuetypes") {
			t.Fatal("ambiguous type led to field read")
		}
		return response(200, `{"startAt":0,"maxResults":100,"total":2,"values":[{"id":"1","name":"Story"},{"id":"2","name":"story"}]}`), nil
	})
	if _, err := c.FetchCreateMeta(context.Background(), "TEST", "Story"); err == nil {
		t.Fatal("silently selected ambiguous type")
	}
}

func TestStatusRegistryRetainsConsumerNameKeys(t *testing.T) {
	c := configured(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/jira/rest/api/2/status":
			return response(200, `[{"id":"1","name":"Open","statusCategory":{"key":"new"}}]`), nil
		case "/jira/rest/api/2/project/TEST/versions":
			return response(200, `[]`), nil
		case "/jira/rest/agile/1.0/board":
			return response(200, `{"startAt":0,"isLast":true,"values":[]}`), nil
		default:
			t.Fatal(r.URL.Path)
			return nil, nil
		}
	})
	dir := t.TempDir()
	if err := legacy.RefreshRegistries(context.Background(), c, "TEST", dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "statuses.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "status:open") || strings.Contains(string(data), "status:1") {
		t.Fatalf("status registry key changed: %s", data)
	}
}
