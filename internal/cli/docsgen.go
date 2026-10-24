package cli

import (
	"fmt"
	"regexp"
	"strings"
)

// Generated regions in README.md and docs/cli.md are delimited by these
// markers. The text between them is owned by the generator.
const (
	generatedBeginPrefix = "<!-- BEGIN GENERATED: "
	generatedBeginSuffix = " -->"
	generatedEnd         = "<!-- END GENERATED -->"
)

// GeneratedRegion renders the named generated region as Markdown (without
// the markers). Known regions: "commands" (summary table) and
// "command-reference" (usage and flags per command).
func GeneratedRegion(name string) (string, error) {
	switch name {
	case "commands":
		return commandsTable(), nil
	case "command-reference":
		return commandReference(), nil
	}
	return "", fmt.Errorf("unknown generated region %q", name)
}

func mdCell(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

func commandsTable() string {
	var b strings.Builder
	b.WriteString("| Command | Description |\n| --- | --- |\n")
	for _, c := range commandDocs {
		if c.Hidden {
			continue
		}
		fmt.Fprintf(&b, "| `jirafs %s` | %s |\n", c.Name, mdCell(c.Summary))
	}
	return b.String()
}

func commandReference() string {
	var b strings.Builder
	first := true
	for _, c := range commandDocs {
		if c.Hidden {
			continue
		}
		if !first {
			b.WriteString("\n")
		}
		first = false
		level := "###"
		if strings.Contains(c.Name, " ") {
			level = "####"
		}
		fmt.Fprintf(&b, "%s `jirafs %s`\n\n%s\n\n```text\n", level, c.Name, c.Summary)
		for _, s := range c.Usage.Synopsis {
			fmt.Fprintf(&b, "jirafs %s\n", s)
		}
		b.WriteString("```\n")
		if len(c.Usage.Description) > 0 {
			b.WriteString("\n" + strings.Join(c.Usage.Description, " ") + "\n")
		}
		for _, s := range c.Usage.Sections {
			fmt.Fprintf(&b, "\n%s:\n\n", s.Title)
			for _, r := range s.Rows {
				if r[1] == "" {
					fmt.Fprintf(&b, "- `%s`\n", r[0])
					continue
				}
				fmt.Fprintf(&b, "- `%s`: %s\n", r[0], r[1])
			}
		}
	}
	return b.String()
}

var beginRe = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(generatedBeginPrefix) + `([a-z-]+)` + regexp.QuoteMeta(generatedBeginSuffix) + `[ \t]*$`)

// RenderGenerated returns doc with the content of every generated region
// replaced by freshly generated text. Text outside the markers is untouched.
// A BEGIN marker without a following END marker is an error.
func RenderGenerated(doc string) (string, error) {
	var out strings.Builder
	rest := doc
	for {
		loc := beginRe.FindStringSubmatchIndex(rest)
		if loc == nil {
			out.WriteString(rest)
			return out.String(), nil
		}
		name := rest[loc[2]:loc[3]]
		body, err := GeneratedRegion(name)
		if err != nil {
			return "", err
		}
		afterBegin := loc[1]
		if afterBegin < len(rest) && rest[afterBegin] == '\n' {
			afterBegin++
		}
		end := strings.Index(rest[afterBegin:], generatedEnd)
		if end < 0 {
			return "", fmt.Errorf("generated region %q has no %s marker", name, generatedEnd)
		}
		out.WriteString(rest[:loc[1]])
		out.WriteString("\n")
		out.WriteString(body)
		rest = rest[afterBegin+end:]
	}
}
