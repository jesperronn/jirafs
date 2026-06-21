package registry

// FixVersion represents a Jira fix-version as stored in the fix-version registry.
type FixVersion struct {
	// Name is the version string (e.g. "1.4.0").
	Name string `yaml:"name"`
	// Description is an optional free-text description.
	Description string `yaml:"description,omitempty"`
	// Archived indicates whether the version is archived.
	Archived bool `yaml:"archived,omitempty"`
	// Released indicates whether the version has been released.
	Released bool `yaml:"released,omitempty"`
}

// IsZero reports whether f has no fix-version data set.
func (f FixVersion) IsZero() bool {
	return f.Name == "" && f.Description == "" && !f.Archived && !f.Released
}
