// Package jira provides generic issue read/write access to Jira Data Center REST API v2.
package jira

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTimeout = 30 * time.Second
	maxAttempts    = 4
	maxRetryDelay  = time.Minute
	maxBodyBytes   = 16 << 20
)

// Config configures a client for one Data Center instance, including an
// optional context path. A nil Transport uses the system TLS trust store.
type Config struct {
	BaseURL      string
	Credentials  Credentials
	Timeout      time.Duration
	Transport    http.RoundTripper
	CustomFields []string
}

// Client is safe for concurrent calls. Its configuration is copied at creation.
// Mutations use explicit typed methods; no arbitrary route is exposed.
type Client struct {
	baseURL      *url.URL
	httpClient   *http.Client
	credentials  Credentials
	customFields []string
	sleep        func(context.Context, time.Duration) error
}

var customFieldPattern = regexp.MustCompile(`^customfield_[0-9]+$`)

// NewClient validates configuration without contacting Jira.
func NewClient(cfg Config) (*Client, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("Jira base URL must be an HTTP(S) URL without credentials, query or fragment")
	}
	if err := cfg.Credentials.validate(); err != nil {
		return nil, err
	}
	if cfg.Timeout < 0 {
		return nil, errors.New("Jira timeout must be positive")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	fields := make([]string, 0, len(cfg.CustomFields))
	seen := make(map[string]bool)
	for _, field := range cfg.CustomFields {
		if !customFieldPattern.MatchString(field) {
			return nil, errors.New("Jira custom fields must use customfield_ followed by digits")
		}
		if !seen[field] {
			fields = append(fields, field)
			seen[field] = true
		}
	}
	return &Client{
		baseURL: u, credentials: cfg.Credentials, customFields: fields, sleep: wait,
		httpClient: &http.Client{
			Transport: cfg.Transport, Timeout: cfg.Timeout,
			// Login redirects must not turn into HTML successes or forward auth.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

// TransportError classifies a failed read without retaining sensitive URLs or
// provider messages. Its cause is sanitized; context errors remain inspectable.
type TransportError struct{ cause error }

func (e *TransportError) Error() string { return "Jira read transport failed" }
func (e *TransportError) Unwrap() error { return e.cause }

// HTTPError identifies a non-200 response without exposing its URL or body.
type HTTPError struct {
	StatusCode int
	Attempts   int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Jira returned HTTP %d after %d attempt(s)", e.StatusCode, e.Attempts)
}

func wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.ParseInt(header, 10, 64); err == nil && seconds >= 0 {
		if seconds >= int64(maxRetryDelay/time.Second) {
			return maxRetryDelay
		}
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(header); err == nil {
		return min(max(time.Until(date), 0), maxRetryDelay)
	}
	return time.Second << (attempt - 1)
}

// get only accepts internally constructed read URLs. It never follows redirects.
func (c *Client) get(ctx context.Context, u *url.URL) ([]byte, error) {
	return c.read(ctx, http.MethodGet, u, nil)
}

// read also supports POST-based JQL search, which does not mutate Jira.
// The complete body is recreated for each retry; callers supply fixed read routes.
func (c *Client) read(ctx context.Context, method string, u *url.URL, payload []byte) ([]byte, error) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(payload))
		if err != nil {
			return nil, errors.New("cannot construct Jira read request")
		}
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		c.credentials.authenticate(req)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, context.DeadlineExceeded
			}
			// A transport error may include credentials or a sensitive URL.
			return nil, &TransportError{cause: errors.New("request failed")}
		}
		if (resp.StatusCode == 429 || resp.StatusCode == 503) && attempt < maxAttempts {
			delay := retryDelay(resp.Header.Get("Retry-After"), attempt)
			resp.Body.Close()
			if err := c.sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, &HTTPError{StatusCode: resp.StatusCode, Attempts: attempt}
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
		resp.Body.Close()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, context.DeadlineExceeded
			}
			return nil, &TransportError{cause: errors.New("response read failed")}
		}
		if len(body) > maxBodyBytes {
			return nil, errors.New("Jira response exceeds 16 MiB limit")
		}
		return body, nil
	}
	panic("unreachable: every final attempt returns")
}

func (c *Client) issueURL(key string) *url.URL {
	u := c.apiURL("/rest/api/2/issue/" + url.PathEscape(key))
	query := url.Values{"fields": {strings.Join(c.issueFields(), ",")}, "expand": {"names"}}
	u.RawQuery = query.Encode()
	return u
}

// apiURL takes an internally constructed, already escaped REST path.
func (c *Client) apiURL(path string) *url.URL {
	u := *c.baseURL
	u.RawPath = strings.TrimRight(c.baseURL.EscapedPath(), "/") + path
	u.Path, _ = url.PathUnescape(u.RawPath)
	return &u
}

func (c *Client) issueFields() []string {
	fields := []string{"summary", "description", "project", "issuetype", "status", "updated", "fixVersions", "issuelinks",
		"labels", "assignee", "priority", "reporter", "created", "components", "parent", "subtasks"}
	return append(fields, c.customFields...)
}
