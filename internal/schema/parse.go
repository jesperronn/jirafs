package schema

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ParseErrorKind identifies the category of a frontmatter parse failure.
type ParseErrorKind string

const (
	// ErrKindNoFrontmatter means the content had no frontmatter delimiters.
	ErrKindNoFrontmatter ParseErrorKind = "no_frontmatter"
	// ErrKindNoClosingDelimiter means the frontmatter had no closing delimiter.
	ErrKindNoClosingDelimiter ParseErrorKind = "no_closing_delimiter"
	// ErrKindInvalidYAML means the frontmatter was not valid YAML.
	ErrKindInvalidYAML ParseErrorKind = "invalid_yaml"
	// ErrKindInvalidProjectRef means the project field was not a valid typed ref.
	ErrKindInvalidProjectRef ParseErrorKind = "invalid_project_ref"
	// ErrKindInvalidSyncTime means the sync_time field was not a valid RFC3339 date.
	ErrKindInvalidSyncTime ParseErrorKind = "invalid_sync_time"
)

// ParseError is a structured error returned by ParseIssue when frontmatter
// validation fails. The Kind field identifies the failure category so callers
// can handle specific error conditions programmatically.
type ParseError struct {
	Kind ParseErrorKind
	Msg  string
}

func (e *ParseError) Error() string {
	return string(e.Kind) + ": " + e.Msg
}

// ParseIssue parses the YAML frontmatter of an issue file content string
// and returns a populated Issue. It handles synced and draft issue
// frontmatter including identity, machine-owned fields, remote metadata,
// and state.
//
// The frontmatter is expected to be delimited by "---" at the start and
// end of the content. Everything between the delimiters is parsed as YAML
// into the Issue's Identity, MachineOwned, RemoteMetadata, and State fields.
//
// On failure, ParseIssue returns a *ParseError whose Kind identifies the
// failure category (ErrKindNoFrontmatter, ErrKindInvalidYAML, etc.).
func ParseIssue(content string) (Issue, *ParseError) {
	var issue Issue

	// Extract frontmatter from the content string.
	frontmatter, body, pe := extractFrontmatter(content)
	if pe != nil {
		return Issue{}, pe
	}

	if frontmatter == "" {
		return issue, &ParseError{
			Kind: ErrKindNoFrontmatter,
			Msg:  "no frontmatter found",
		}
	}

	// Parse the identity fields.
	var rawIdentity struct {
		Key     string `yaml:"key"`
		Type    string `yaml:"type"`
		Project string `yaml:"project"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &rawIdentity); err != nil {
		return Issue{}, &ParseError{
			Kind: ErrKindInvalidYAML,
			Msg:  err.Error(),
		}
	}

	issue.Identity = IssueIdentity{
		Key:     IssueKey(rawIdentity.Key),
		Type:    IssueType(rawIdentity.Type),
		Project: TypedRef{},
	}

	if rawIdentity.Project != "" {
		pr, err := ParseTypedRef(rawIdentity.Project)
		if err != nil {
			return Issue{}, &ParseError{
				Kind: ErrKindInvalidProjectRef,
				Msg:  err.Error(),
			}
		}
		issue.Identity.Project = pr
	}

	// Parse machine-owned fields.
	var rawMachine struct {
		SchemaVersion string `yaml:"schema_version"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &rawMachine); err != nil {
		return Issue{}, &ParseError{
			Kind: ErrKindInvalidYAML,
			Msg:  err.Error(),
		}
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
		return Issue{}, &ParseError{
			Kind: ErrKindInvalidYAML,
			Msg:  err.Error(),
		}
	}
	issue.RemoteMetadata.StateFile = rawState.State
	if rawState.RemoteVersion != "" || rawState.ContentHash != "" || rawState.SyncTime != "" {
		var syncTime time.Time
		var parseErr error
		if rawState.SyncTime != "" {
			syncTime, parseErr = time.Parse(time.RFC3339, rawState.SyncTime)
			if parseErr != nil {
				return Issue{}, &ParseError{
					Kind: ErrKindInvalidSyncTime,
					Msg:  fmt.Sprintf("invalid sync_time %q: %s", rawState.SyncTime, parseErr.Error()),
				}
			}
		}
		issue.RemoteMetadata = RemoteMetadata{
			RemoteVersion: rawState.RemoteVersion,
			ContentHash:   rawState.ContentHash,
			SyncTime:      syncTime,
			StateFile:     rawState.State,
		}
	}

	// Populate Sections from body when present.
	if body != "" {
		blocks := splitSectionBlocks(body)
		issue.Sections = make(map[FixedSectionName]string)
		for _, b := range blocks {
			name := FixedSectionName(b.Heading)
			issue.Sections[name] = b.Body
		}
		// Ensure Description and Acceptance Criteria always exist.
		if _, ok := issue.Sections[SecDescription]; !ok {
			issue.Sections[SecDescription] = ""
		}
		if _, ok := issue.Sections[SecAcceptanceCriteria]; !ok {
			issue.Sections[SecAcceptanceCriteria] = ""
		}
	}

	return issue, nil
}

type sectionBlock struct {
	Heading string
	Body    string
}

// extractFrontmatter extracts the YAML frontmatter from the content string.
// It returns the content between the opening and closing "---" delimiters,
// or a *ParseError if no valid frontmatter is found.
func extractFrontmatter(content string) (string, string, *ParseError) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return "", "", &ParseError{
			Kind: ErrKindNoFrontmatter,
			Msg:  "no frontmatter delimiter",
		}
	}

	// Find the closing delimiter.
	rest := trimmed[3:]
	idx := strings.Index(rest, "---")
	if idx < 0 {
		return "", "", &ParseError{
			Kind: ErrKindNoClosingDelimiter,
			Msg:  "no closing frontmatter delimiter",
		}
	}

	frontmatter := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(rest[idx+3:])
	return frontmatter, body, nil
}

func splitSectionBlocks(body string) []sectionBlock {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	lines := strings.Split(body, "\n")
	var blocks []sectionBlock
	var current *sectionBlock

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current != nil {
				current.Body = strings.TrimSpace(current.Body)
				blocks = append(blocks, *current)
			}
			current = &sectionBlock{
				Heading: strings.TrimSpace(strings.TrimPrefix(line, "## ")),
			}
			continue
		}
		if current == nil {
			continue
		}
		if current.Body == "" {
			current.Body = line
			continue
		}
		current.Body += "\n" + line
	}

	if current != nil {
		current.Body = strings.TrimSpace(current.Body)
		blocks = append(blocks, *current)
	}

	return blocks
}
