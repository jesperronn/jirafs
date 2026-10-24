package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// intentLabelPrefix prefixes the token label added to an issue while it is
// being created. Jira labels cannot contain spaces; hex tokens are safe.
const intentLabelPrefix = "jirafs-"

// staleIntentAge is how old an intent record may get before doctor reports it.
const staleIntentAge = 7 * 24 * time.Hour

// createIntent records that a draft is about to be created in Jira. It is
// written before the create call so a retry can find an issue whose response
// was lost, by searching for the label jirafs-<Token>.
type createIntent struct {
	Token     string    `yaml:"token"`
	Project   string    `yaml:"project"`
	Summary   string    `yaml:"summary"`
	Timestamp time.Time `yaml:"timestamp"`
}

// Label is the Jira label that marks the issue created for this intent.
func (i createIntent) Label() string { return intentLabelPrefix + i.Token }

// JQL is the search that finds the issue created for this intent.
func (i createIntent) JQL() string { return fmt.Sprintf("labels = %q", i.Label()) }

func intentDir(mirrorDir string) string {
	return filepath.Join(mirrorDir, ".jirafs", "intents")
}

// intentPath is the record for the draft file at draftPath.
func intentPath(mirrorDir, draftPath string) string {
	base := strings.TrimSuffix(filepath.Base(draftPath), filepath.Ext(draftPath))
	return filepath.Join(intentDir(mirrorDir), base+".yml")
}

// newIntentToken returns a random 16-hex-digit token.
func newIntentToken() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// loadIntent reads the record at path; ok is false when there is none or it
// is unreadable or has no token (a damaged record is replaced by a new one).
func loadIntent(path string) (createIntent, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return createIntent{}, false
	}
	var in createIntent
	if yaml.Unmarshal(data, &in) != nil || in.Token == "" {
		return createIntent{}, false
	}
	return in, true
}

func saveIntent(path string, in createIntent) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(in)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// staleIntents lists the intent files under mirrorDir older than 7 days, as
// "name (age)" lines, sorted by name.
func staleIntents(mirrorDir string, now time.Time) []string {
	entries, err := os.ReadDir(intentDir(mirrorDir))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		in, ok := loadIntent(filepath.Join(intentDir(mirrorDir), e.Name()))
		if !ok || now.Sub(in.Timestamp) <= staleIntentAge {
			continue
		}
		days := int(now.Sub(in.Timestamp).Hours() / 24)
		out = append(out, fmt.Sprintf("%s: %d days old, label %s; run: jirafs push %s --apply",
			e.Name(), days, in.Label(), filepath.Join(mirrorDir, strings.TrimSuffix(e.Name(), ".yml")+".md")))
	}
	sort.Strings(out)
	return out
}
