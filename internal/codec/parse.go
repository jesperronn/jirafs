package codec

import (
	"fmt"
	"strings"
	"time"

	"github.com/jirafs/jirafs/internal/schema"
	"gopkg.in/yaml.v3"
)

// ParseIssue parses an issue Markdown document into a structured model.
// It handles both synced and draft issues, parsing the frontmatter and
// splitting the body into sections.
func ParseIssue(content string) (*schema.Issue, error) {
	// Split the content into frontmatter and body
	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Parse the frontmatter into a schema model
	issue, err := parseFrontmatter(frontmatter)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Parse body into sections if needed
	if body != "" {
		sections, err := parseSections(body)
		if err != nil {
			return nil, fmt.Errorf("failed to parse sections: %w", err)
		}
		issue.Sections = sections
	}

	return issue, nil
}

// splitFrontmatter separates the YAML frontmatter from the rest of the document.
func splitFrontmatter(content string) (string, string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", content, nil
	}

	// Find the end of the frontmatter (second ---)
	endFrontmatter := strings.Index(content[3:], "---")
	if endFrontmatter == -1 {
		return "", content, fmt.Errorf("malformed frontmatter: missing closing ---")
	}

	endFrontmatter += 3 // Adjust for the offset
	frontmatter := content[0:endFrontmatter+3]
	body := strings.TrimSpace(content[endFrontmatter+3:])

	return frontmatter, body, nil
}

// parseFrontmatter parses the YAML frontmatter into a schema.Issue.
func parseFrontmatter(frontmatter string) (*schema.Issue, error) {
	// Remove the opening and closing --- markers
	frontmatter = strings.TrimPrefix(frontmatter, "---")
	frontmatter = strings.TrimSuffix(frontmatter, "---")
	frontmatter = strings.TrimSpace(frontmatter)

	// Parse YAML into a map
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontmatter), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Create a new issue instance
	issue := &schema.Issue{
		Sections: make(map[schema.FixedSectionName]string),
	}

	// Handle machine-owned fields
	if schemaVersion, ok := raw["schema_version"]; ok {
		issue.MachineOwned.SchemaVersion = schemaVersion.(string)
	}

	// Handle identity fields
	if key, ok := raw["key"]; ok {
		issue.Identity.Key = schema.IssueKey(key.(string))
	}
	if issueType, ok := raw["type"]; ok {
		issue.Identity.Type = schema.IssueType(issueType.(string))
	}
	if project, ok := raw["project"]; ok {
		// For now, we just store it as-is - actual resolution will happen later
		issue.Identity.Project = parseTypedRef(project)
	}

	// Handle remote metadata fields
	if version, ok := raw["remote_version"]; ok {
		issue.RemoteMetadata.RemoteVersion = version.(string)
	}
	if contentHash, ok := raw["content_hash"]; ok {
		issue.RemoteMetadata.ContentHash = contentHash.(string)
	}
	if stateFile, ok := raw["state"]; ok {
		issue.RemoteMetadata.StateFile = stateFile.(string)
	}
	if syncTime, ok := raw["sync_time"]; ok {
		if t, err := time.Parse(time.RFC3339, syncTime.(string)); err == nil {
			issue.RemoteMetadata.SyncTime = t
		}
	}
	if resolvedStatus, ok := raw["resolved_status"]; ok {
		issue.RemoteMetadata.ResolvedStatus = resolvedStatus.(string)
	}
	if pinned, ok := raw["pinned"]; ok {
		issue.RemoteMetadata.Pinned = pinned.(bool)
	}

	// Handle editable fields
	if summary, ok := raw["summary"]; ok {
		issue.Summary = summary.(string)
	}
	if description, ok := raw["description"]; ok {
		issue.Description = description.(string)
	}
	if labels, ok := raw["labels"]; ok {
		labelsList := labels.([]interface{})
		issue.Labels = make([]string, len(labelsList))
		for i, label := range labelsList {
			issue.Labels[i] = label.(string)
		}
	}
	if assignee, ok := raw["assignee"]; ok {
		assigneeStr := assignee.(string)
		issue.Assignee = &assigneeStr
	}
	if status, ok := raw["status"]; ok {
		issue.Status = status.(string)
	}
	if sprint, ok := raw["sprint"]; ok {
		issue.Sprint = sprint.(string)
	}
	if fixVersions, ok := raw["fix_versions"]; ok {
		fixVersionList := fixVersions.([]interface{})
		issue.FixVersions = make([]string, len(fixVersionList))
		for i, fixVersion := range fixVersionList {
			issue.FixVersions[i] = fixVersion.(string)
		}
	}

	// Handle linked issues
	if linkedIssues, ok := raw["linked_issues"]; ok {
		issue.LinkedIssues = parseLinkedIssues(linkedIssues)
	}

	return issue, nil
}

// parseTypedRef parses a typed reference (e.g., "user://johndoe" or "project://PROJ")
func parseTypedRef(ref interface{}) schema.TypedRef {
	if ref == nil {
		return schema.TypedRef{}
	}

	refStr := ref.(string)
	parts := strings.SplitN(refStr, ":", 2)
	if len(parts) != 2 {
		return schema.TypedRef{Type: "unknown", Value: refStr}
	}

	return schema.TypedRef{
		Type:  schema.RefType(parts[0]),
		Value: parts[1],
	}
}

// parseLinkedIssues parses linked issue references from YAML.
func parseLinkedIssues(linkedIssues interface{}) []schema.LinkedIssue {
	if linkedIssues == nil {
		return []schema.LinkedIssue{}
	}

	issuesList, ok := linkedIssues.([]interface{})
	if !ok {
		return []schema.LinkedIssue{}
	}

	result := make([]schema.LinkedIssue, 0, len(issuesList))

	for _, item := range issuesList {
		issueMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := issueMap["key"].(string)
		liType, _ := issueMap["type"].(string)
		result = append(result, schema.LinkedIssue{
			Key:  schema.IssueKey(key),
			Type: liType,
		})
	}

	return result
}

// parseSections splits the body into sections by ## headers.
func parseSections(body string) (map[schema.FixedSectionName]string, error) {
	sections := make(map[schema.FixedSectionName]string)

	// Normalize line endings and split into lines
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	
	// Keep track of current section name and content
	var currentSectionName schema.FixedSectionName
	var currentSectionContent strings.Builder
	
	for i, line := range lines {
		line = strings.TrimSpace(line)
		
		// Check if this is a new section header (## Something)
		if strings.HasPrefix(line, "## ") {
			// Save the previous section if we had one
			if currentSectionName != "" {
				sections[currentSectionName] = strings.TrimSpace(currentSectionContent.String())
			}
			
			// Start new section
			sectionName := strings.TrimSpace(line[3:]) // Remove "## "
			
			// Check if it's a known fixed section name
			fixedSectionName := schema.FixedSectionName(sectionName)
			if !fixedSectionName.IsKnown() {
				return nil, fmt.Errorf("unknown section name: %s", sectionName)
			}
			
			currentSectionName = fixedSectionName
			currentSectionContent.Reset()
		} else if currentSectionName != "" {
			// Add content to current section
			if i > 0 {
				currentSectionContent.WriteString("\n")
			}
			currentSectionContent.WriteString(line)
		}
	}
	
	// Save the last section if there was one
	if currentSectionName != "" {
		sections[currentSectionName] = strings.TrimSpace(currentSectionContent.String())
	}
	
	return sections, nil
}