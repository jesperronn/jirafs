package registry

// Status represents a Jira issue status as stored in the status registry.
type Status struct {
	// Name is the human-readable status name (e.g. "In Progress").
	Name string `yaml:"name"`
	// Category is the status category (e.g. "InProgress", "Done", "ToDos").
	Category string `yaml:"category,omitempty"`
	// Description is an optional free-text description.
	Description string `yaml:"description,omitempty"`
}

// IsZero reports whether s has no status data set.
func (s Status) IsZero() bool {
	return s.Name == "" && s.Category == "" && s.Description == ""
}
