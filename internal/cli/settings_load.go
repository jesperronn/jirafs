package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jirafs/jirafs/internal/config"
)

const (
	defaultSettingsPath = "~/.jirafs/settings.toml"

	// firstRunSetupCommand is the copy-pasteable command shown when no
	// settings exist yet.
	firstRunSetupCommand = "jirafs setup --project <name> --key <KEY> --instance <name> " +
		"--base-url https://<site>.atlassian.net --credential-ref env://JIRAFS_API_TOKEN --set-current"
)

// loadSettingsOrHint loads the jirafs settings for command cmd. On success it
// returns the settings and true. On failure it writes a diagnostic to stderr
// and returns nil and false; the caller should exit with code 1.
//
// A missing settings file (or missing required field) produces a first-run
// message with a copy-pasteable setup command. Other failures, such as an
// unreadable or unparsable file, keep the underlying error and name the file.
func loadSettingsOrHint(stderr io.Writer, cmd string) (*config.Settings, bool) {
	s, err := config.Load()
	if err == nil {
		return s, true
	}
	if isSettingsMissing(err) {
		fmt.Fprintf(stderr, "jirafs %s: no settings found (%s)\n", cmd, defaultSettingsPath)
		fmt.Fprintf(stderr, "next step: %s\n", firstRunSetupCommand)
		return nil, false
	}
	fmt.Fprintf(stderr, "jirafs %s: cannot load settings: %v (file: %s)\n", cmd, err, settingsErrorPath(err))
	return nil, false
}

// isSettingsMissing reports whether err means the settings file or a required
// field is absent, as opposed to the file being unreadable or malformed.
func isSettingsMissing(err error) bool {
	if !config.IsSettingError(err, config.ErrMissingField) {
		return false
	}
	var se *config.SettingError
	if errors.As(err, &se) {
		if se.Field == "file" || strings.HasPrefix(se.Message, "cannot read settings file") {
			return false
		}
	}
	return true
}

// settingsErrorPath returns the settings file path recorded in err, or the
// default location when the error does not carry one.
func settingsErrorPath(err error) string {
	var se *config.SettingError
	if errors.As(err, &se) && se.Value != "" {
		return se.Value
	}
	return defaultSettingsPath
}
