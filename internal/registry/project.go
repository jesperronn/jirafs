package registry

// Project represents a Jira project as stored in the project registry.
type Project struct {
	Key     string `yaml:"key"`
	Name    string `yaml:"name"`
	ID      string `yaml:"id"`
	Avatar  string `yaml:"avatar,omitempty"`
	Lead    string `yaml:"lead,omitempty"`
	ProjectType string `yaml:"project_type,omitempty"`
}

// IsZero reports whether p has no project data set.
func (p Project) IsZero() bool {
	return p.Key == "" && p.Name == "" && p.ID == ""
}
