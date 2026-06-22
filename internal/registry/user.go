package registry

// User represents a Jira user as stored in the user registry.
type User struct {
	AccountID  string `yaml:"account_id"`
	DisplayName string `yaml:"display_name"`
	Email      string `yaml:"email"`
	Active     bool   `yaml:"active"`
}

// IsZero reports whether u has no user data set.
func (u User) IsZero() bool {
	return u.AccountID == "" && u.DisplayName == "" && u.Email == "" && !u.Active
}
