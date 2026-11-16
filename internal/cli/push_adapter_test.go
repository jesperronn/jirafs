package cli

import (
	stdcontext "context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jiraconnect"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

type adapterHTTP func(*http.Request) (*http.Response, error)

func (f adapterHTTP) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func adapterResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func adapterClient(rt adapterHTTP) *jiraconnect.Client {
	c := jiraconnect.New(jiraconnect.Config{BaseURL: "https://jira.example.test", Transport: rt})
	c.SetCredentials(config.ResolvedInstanceCredentials{AuthType: "bearer_token", Credential: config.ResolvedCredential{Fields: map[string]string{"bearer_token": "offline"}}})
	return c
}

func TestPushConnectorWriteOutcomesDoNotReplayOrAdvanceLocalState(t *testing.T) {
	for _, mode := range []string{"refresh", "unknown", "transition", "rejected"} {
		t.Run(mode, func(t *testing.T) {
			out, errBuf := withPushIO(t)
			puts, transitions := 0, 0
			accepted := false
			client := adapterClient(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPut:
					puts++
					if mode == "unknown" {
						return nil, errors.New("offline lost response")
					}
					if mode == "rejected" {
						return adapterResponse(403, `{}`), nil
					}
					accepted = true
					return adapterResponse(204, ""), nil
				case req.Method == http.MethodPost:
					transitions++
					return nil, errors.New("offline lost transition response")
				case strings.HasSuffix(req.URL.Path, "/transitions"):
					return adapterResponse(200, `{"transitions":[{"id":"11","name":"Start","to":{"id":"2","name":"In Progress"}}]}`), nil
				default:
					if accepted && mode == "refresh" {
						return adapterResponse(403, `{}`), nil
					}
					summary := "before"
					if accepted {
						summary = "after"
					}
					return adapterResponse(200, `{"id":"42","key":"PROJ-42","fields":{"summary":"`+summary+`","description":"body","labels":[],"issuetype":{"name":"Story"},"project":{"key":"PROJ"},"status":{"name":"Open"},"updated":"2026-01-01T00:00:00Z"}}`), nil
				}
			})
			base, err := client.FetchIssue(stdcontext.Background(), "PROJ-42")
			if err != nil {
				t.Fatal(err)
			}
			base.Context.Status = base.Status
			mirrorDir := t.TempDir()
			if err := mirror.SaveBase(mirrorDir, base); err != nil {
				t.Fatal(err)
			}
			local := *base
			local.Summary = "after"
			if mode == "refresh" || mode == "transition" {
				local.Context.Status = "In Progress"
			}
			path := filepath.Join(t.TempDir(), "PROJ-42.md")
			doc := schema.RenderIssue(local)
			if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
				t.Fatal(err)
			}
			basePath := mirror.BasePath(mirrorDir, base.Identity.Key)
			before, err := os.ReadFile(basePath)
			if err != nil {
				t.Fatal(err)
			}
			run := &pushRun{cmdCtx: stdcontext.Background(), ctx: &context.Context{Name: "test", MirrorDir: mirrorDir}, opts: pushOptions{apply: true, writeLocal: true, project: "test", cwd: "/tmp/offline project"}, madeCli: true, client: client}
			result := run.updateIssue("PROJ-42", local, path)
			if printPushSummary([]pushResult{result}) != 1 {
				t.Fatal("incomplete push must require attention")
			}
			text := out.String() + errBuf.String()
			want := "outcome unknown"
			if mode == "refresh" {
				want = "accepted"
			}
			if mode == "rejected" {
				want = "cannot update remote issue"
			}
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q: %s", want, text)
			}
			if mode == "refresh" && strings.Contains(text, "cannot update remote issue") {
				t.Fatalf("accepted write called rejected: %s", text)
			}
			if mode != "rejected" && !strings.Contains(text, "jirafs pull PROJ-42") {
				t.Fatalf("missing read-only refresh command: %s", text)
			}
			if mode == "transition" && !strings.Contains(text, "fields already applied") {
				t.Fatalf("partial write omitted: %s", text)
			}
			if puts != 1 {
				t.Fatalf("PUT count=%d", puts)
			}
			wantTransitions := 0
			if mode == "transition" {
				wantTransitions = 1
			}
			if transitions != wantTransitions {
				t.Fatalf("transition count=%d", transitions)
			}
			after, _ := os.ReadFile(basePath)
			if string(before) != string(after) {
				t.Fatal("base advanced after incomplete write")
			}
			localAfter, _ := os.ReadFile(path)
			if string(localAfter) != doc {
				t.Fatal("local file changed after incomplete write")
			}
		})
	}
}

func TestPushConnectorUnknownCreateRetainsDraftAndIntent(t *testing.T) {
	dir := setupNewTest(t)
	path := writeDraft(t, dir)
	doc, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	posts, searches := 0, 0
	client := adapterClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/search") {
			searches++
			return adapterResponse(200, `{"startAt":0,"maxResults":100,"total":0,"issues":[]}`), nil
		}
		if req.Method == http.MethodPost {
			posts++
			return nil, errors.New("offline lost create response")
		}
		if strings.HasSuffix(req.URL.Path, "/issuetypes") {
			return adapterResponse(200, `{"startAt":0,"maxResults":100,"total":1,"values":[{"id":"1","name":"Story"}]}`), nil
		}
		return adapterResponse(200, `{"startAt":0,"maxResults":100,"total":0,"values":[]}`), nil
	})
	out, errBuf := withPushIO(t)
	mirrorDir := t.TempDir()
	run := &pushRun{cmdCtx: stdcontext.Background(), ctx: &context.Context{Name: "test", Instance: "work", Key: "TEST", MirrorDir: mirrorDir}, settings: &config.Settings{Instances: map[string]config.Instance{"work": {BaseURL: "https://jira.example.test"}}, Projects: map[string]config.Project{"test": {}}}, opts: pushOptions{apply: true, project: "test", cwd: dir}, madeCli: true, client: client}
	for attempt := 0; attempt < 2; attempt++ {
		result := run.createDraft(path, string(doc))
		if printPushSummary([]pushResult{result}) != 1 {
			t.Fatal("unknown create must require attention")
		}
		if !strings.Contains(out.String()+errBuf.String(), "outcome unknown") {
			t.Fatalf("unknown creation not classified: %s %s", out, errBuf)
		}
		got, _ := os.ReadFile(path)
		if string(got) != string(doc) {
			t.Fatal("unknown create changed draft")
		}
		if _, ok := loadIntent(intentPath(mirrorDir, path)); !ok {
			t.Fatal("create intent was removed")
		}
	}
	if posts != 2 || searches != 1 {
		t.Fatalf("explicit second run must search intent before its own one create; posts=%d searches=%d", posts, searches)
	}
}
