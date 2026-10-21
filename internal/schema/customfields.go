package schema

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var customFieldKey = regexp.MustCompile(`^customfield_[0-9]+$`)

// IsCustomFieldKey reports whether k looks like customfield_NNNNN.
func IsCustomFieldKey(k string) bool { return customFieldKey.MatchString(k) }

// NormalizeCustomFields validates a decoded `custom_fields:` value. Keys must
// be customfield_NNNNN; values (string, number, bool, list, or map such as
// {value: "X"} / {id: "123"}) are passed through to Jira as JSON unchanged.
func NormalizeCustomFields(raw interface{}) (map[string]interface{}, error) {
	if raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("custom_fields must be a map of customfield_NNNNN keys")
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if !IsCustomFieldKey(k) {
			return nil, fmt.Errorf("custom_fields key %q must look like customfield_12345", k)
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// MergeCustomFields returns defaults overlaid by own: the draft's own values
// win over defaults. Nil when both are empty.
func MergeCustomFields(defaults, own map[string]interface{}) map[string]interface{} {
	if len(defaults) == 0 && len(own) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(defaults)+len(own))
	for k, v := range defaults {
		out[k] = v
	}
	for k, v := range own {
		out[k] = v
	}
	return out
}

// renderCustomFields renders the custom_fields frontmatter block with sorted
// keys, or "" when empty.
func renderCustomFields(cf map[string]interface{}) string {
	if len(cf) == 0 {
		return ""
	}
	b, err := yaml.Marshal(cf)
	if err != nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("custom_fields:\n")
	for _, l := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		sb.WriteString("  " + l + "\n")
	}
	return sb.String()
}
