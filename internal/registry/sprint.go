package registry

import "time"

// Sprint represents a Jira sprint as stored in the sprint registry.
type Sprint struct {
	// ID is the Jira-assigned numeric ID.
	ID int64 `yaml:"id"`
	// Name is the human-readable sprint name.
	Name string `yaml:"name"`
	// State is the sprint lifecycle state (e.g. "active", "closed", "future").
	State string `yaml:"state,omitempty"`
	// StartDate is when the sprint begins.
	StartDate *time.Time `yaml:"start_date,omitempty"`
	// EndDate is when the sprint ends.
	EndDate *time.Time `yaml:"end_date,omitempty"`
	// CompleteDate is when the sprint was completed.
	CompleteDate *time.Time `yaml:"complete_date,omitempty"`
}

// IsZero reports whether s has no sprint data set.
func (s Sprint) IsZero() bool {
	return s.ID == 0 && s.Name == "" && s.State == ""
}
