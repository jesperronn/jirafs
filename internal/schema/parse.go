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
	frontmatter, pe := extractFrontmatter(content)
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

	// Parse the body content after the frontmatter.
	issue.Sections = parseSections(extractBody(content))

	return issue, nil
}

// parseSections extracts body sections from content following the closing
// "---" delimiter. It finds lines starting with "## " and treats the
// section name (trimmed) as a FixedSectionName. The content between
// consecutive headers (or end of content) becomes the section body,
// with leading and trailing blank lines stripped.
func parseSections(content string) map[FixedSectionName]string {
	sections := make(map[FixedSectionName]string)
	lines := strings.Split(content, "\n")
	var current FixedSectionName
	var buf []string

	flush := func() {
		if current == "" {
			return
		}
		// Join buffer lines, strip leading/trailing blank lines.
		result := strings.Join(buf, "\n")
		result = strings.TrimRight(result, "\n")
		// Strip leading blank lines.
		for len(result) > 0 && result[0] == '\n' {
			result = result[1:]
		}
		// Strip trailing blank lines.
		for len(result) > 0 && result[len(result)-1] == '\n' {
			result = result[:len(result)-1]
		}
		sections[current] = result
		current = ""
		buf = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			name := FixedSectionName(strings.TrimSpace(line[3:]))
			if name.IsKnown() {
				current = name
			}
		} else {
			buf = append(buf, line)
		}
	}
	flush()
	return sections
}

// extractFrontmatter extracts the YAML frontmatter from the content string.
// It returns the content between the opening and closing "---" delimiters,
// or a *ParseError if no valid frontmatter is found.
func extractFrontmatter(content string) (string, *ParseError) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return "", &ParseError{
			Kind: ErrKindNoFrontmatter,
			Msg:  "no frontmatter delimiter",
		}
	}

	// Find the closing delimiter.
	rest := trimmed[3:]
	idx := strings.Index(rest, "---")
	if idx < 0 {
		return "", &ParseError{
			Kind: ErrKindNoClosingDelimiter,
			Msg:  "no closing frontmatter delimiter",
		}
	}

	return strings.TrimSpace(rest[:idx]), nil
}

// extractBody returns the content after the closing "---" frontmatter
// delimiter, with surrounding whitespace trimmed.
func extractBody(content string) string {
	trimmed := strings.TrimSpace(content)
	rest := trimmed[3:] // skip opening "---"
	idx := strings.Index(rest, "---")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(rest[idx+3:])
}
