package schema

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ParseIssue parses the YAML frontmatter of an issue file content string
// and returns a populated Issue. It handles synced and draft issue
// frontmatter including identity, machine-owned fields, remote metadata,
// and state.
//
// The frontmatter is expected to be delimited by "---" at the start and
// end of the content. Everything between the delimiters is parsed as YAML
// into the Issue's Identity, MachineOwned, RemoteMetadata, and State fields.
func ParseIssue(content string) (Issue, error) {
	var issue Issue

	// Extract frontmatter from the content string.
	frontmatter, err := extractFrontmatter(content)
	if err != nil {
		return Issue{}, fmt.Errorf("parse issue: %w", err)
	}

	if frontmatter == "" {
		return issue, fmt.Errorf("parse issue: no frontmatter found")
	}

	// Parse the identity fields.
	var rawIdentity struct {
		Key     string `yaml:"key"`
		Type    string `yaml:"type"`
		Project string `yaml:"project"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &rawIdentity); err != nil {
		return Issue{}, fmt.Errorf("parse issue: invalid YAML: %w", err)
	}

	issue.Identity = IssueIdentity{
		Key:     IssueKey(rawIdentity.Key),
		Type:    IssueType(rawIdentity.Type),
		Project: TypedRef{},
	}

	if rawIdentity.Project != "" {
		pr, err := ParseTypedRef(rawIdentity.Project)
		if err != nil {
			return Issue{}, fmt.Errorf("parse issue: invalid project ref: %w", err)
		}
		issue.Identity.Project = pr
	}

	// Parse machine-owned fields.
	var rawMachine struct {
		SchemaVersion string `yaml:"schema_version"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &rawMachine); err != nil {
		return Issue{}, fmt.Errorf("parse issue: invalid YAML: %w", err)
	}
	issue.MachineOwned = MachineOwned{
		SchemaVersion: rawMachine.SchemaVersion,
	}

	// Parse state and remote metadata.
	var rawState struct {
		State         string `yaml:"state"`
		RemoteVersion string `yaml:"remote_version"`
		ContentHash   string `yaml:"content_hash"`
		SyncTime      string `yaml:"sync_time"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &rawState); err != nil {
		return Issue{}, fmt.Errorf("parse issue: invalid YAML: %w", err)
	}
	issue.RemoteMetadata.StateFile = rawState.State
	if rawState.RemoteVersion != "" || rawState.ContentHash != "" || rawState.SyncTime != "" {
		var syncTime time.Time
		if rawState.SyncTime != "" {
			syncTime, err = time.Parse(time.RFC3339, rawState.SyncTime)
			if err != nil {
				return Issue{}, fmt.Errorf("parse issue: invalid sync_time %q: %w", rawState.SyncTime, err)
			}
		}
		issue.RemoteMetadata = RemoteMetadata{
			RemoteVersion: rawState.RemoteVersion,
			ContentHash:   rawState.ContentHash,
			SyncTime:      syncTime,
			StateFile:     rawState.State,
		}
	}

	return issue, nil
}

// extractFrontmatter extracts the YAML frontmatter from the content string.
// It returns the content between the opening and closing "---" delimiters,
// or an error if no valid frontmatter is found.
func extractFrontmatter(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return "", fmt.Errorf("no frontmatter delimiter")
	}

	// Find the closing delimiter.
	rest := trimmed[3:]
	idx := strings.Index(rest, "---")
	if idx < 0 {
		return "", fmt.Errorf("no closing frontmatter delimiter")
	}

	return strings.TrimSpace(rest[:idx]), nil
}
