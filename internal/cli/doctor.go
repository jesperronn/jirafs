package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jirafs/jirafs/internal/color"
	"github.com/jirafs/jirafs/internal/config"
	jcontext "github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
)

var (
	doctorStdout   io.Writer = os.Stdout
	doctorStderr   io.Writer = os.Stderr
	doctorClientFactory = buildDoctorClient
)

var doctorVerbose bool

// DoctorSnapshot extends StatusSnapshot with credential and live-probe checks.
// It is consumed by the doctor command to report the health of a jirafs
// workspace: config validity, credential resolution, and live Jira connectivity.
type DoctorSnapshot struct {
	// StatusSnapshot embeds all config and mirror checks.
	StatusSnapshot

	// InstanceCredentials maps instance names to credential check results.
	InstanceCredentials map[string]CredentialCheck

	// LiveProbes maps instance names to live-probe check results.
	LiveProbes map[string]LiveProbeCheck
}

// IsZero reports whether d is the zero value.
func (d DoctorSnapshot) IsZero() bool {
	return d.StatusSnapshot.IsZero() &&
		len(d.InstanceCredentials) == 0 &&
		len(d.LiveProbes) == 0
}

// CredentialCheck holds the result of a credential resolution attempt.
type CredentialCheck struct {
	InstanceName        string
	Resolved            bool
	AuthType            string
	CredentialSummary   string // "scheme://target" summary
	ValidationError     string
}

// LiveProbeCheck holds the result of a live Jira API probe.
type LiveProbeCheck struct {
	InstanceName  string
	URL           string
	HTTPStatus    int
	Connected     bool
	Authenticated bool
	User          string // displayName if authenticated
	Error         string
}

// BuildDoctorSnapshot builds a doctor snapshot for the given settings and
// working directory. It resolves the project (from StatusSnapshot), then
// resolves credentials for each configured instance, and live-probes each
// instance's Jira API via the CurrentUser endpoint.
func BuildDoctorSnapshot(settings *config.Settings, cwd string) DoctorSnapshot {
	dsnap := DoctorSnapshot{}

	// Build the embedded status snapshot first.
	dsnap.StatusSnapshot = BuildStatusSnapshot(settings, cwd)

	dsnap.InstanceCredentials = make(map[string]CredentialCheck)
	dsnap.LiveProbes = make(map[string]LiveProbeCheck)

	if settings == nil {
		return dsnap
	}

	// Resolve credentials for each instance.
	for name, inst := range settings.Instances {
		cc := CredentialCheck{
			InstanceName: name,
			AuthType:     inst.AuthType,
		}

		if len(inst.CredentialRefs) == 0 {
			cc.Resolved = false
			cc.ValidationError = "no credential_refs defined"
		} else {
			creds, err := settings.ResolveInstanceCredentials(name)
			if err != nil {
				cc.Resolved = false
				cc.ValidationError = err.Error()
			} else {
				cc.Resolved = true
				cc.CredentialSummary = fmt.Sprintf("%s (%d field(s))", creds.Credential.Scheme, len(creds.Credential.Fields))
			}
		}

		dsnap.InstanceCredentials[name] = cc
	}

	// Live-probe each instance via CurrentUser.
	for name, inst := range settings.Instances {
		lpc := LiveProbeCheck{
			InstanceName: name,
			URL:          fmt.Sprintf("%s/rest/api/2/myself", strings.TrimRight(inst.BaseURL, "/")),
		}

		// Skip live-probe if credentials did not resolve.
		cc, ok := dsnap.InstanceCredentials[name]
		if !ok || !cc.Resolved {
			lpc.Connected = false
			lpc.Error = "skipped: credential resolution failed"
			dsnap.LiveProbes[name] = lpc
			continue
		}

		// Build a Jira client and probe.
		projCtx := &jcontext.Context{
			Name:     "",
			Key:      "",
			Instance: name,
		}
		client, err := doctorClientFactory(settings, projCtx, cwd)
		if err != nil {
			lpc.Connected = false
			lpc.Error = "cannot create client: " + err.Error()
			dsnap.LiveProbes[name] = lpc
			continue
		}

		// Set credentials on the client.
		creds, _ := settings.ResolveInstanceCredentials(name)
		client.SetCredentials(creds)

		// Probe CurrentUser.
		user, err := client.CurrentUser(context.Background())
		if err != nil {
			lpc.Connected = false
			lpc.Error = err.Error()
			if cerr, ok := err.(*jira.ClientError); ok {
				lpc.HTTPStatus = cerr.HTTPCode
				if cerr.URL != "" {
					lpc.URL = cerr.URL
				}
			}
		} else {
			lpc.Connected = true
			lpc.Authenticated = true
			lpc.User = user.DisplayName
		}

		dsnap.LiveProbes[name] = lpc
	}

	return dsnap
}

func buildDoctorClient(settings *config.Settings, projCtx *jcontext.Context, cwd string) (jira.Client, error) {
	creds, err := settings.ResolveInstanceCredentials(projCtx.Instance)
	if err != nil {
		return nil, err
	}
	client := jira.NewJiraClient(creds.BaseURL)
	client.SetCredentials(creds)
	return client, nil
}

// RunDoctor dispatches the `jirafs doctor` subcommand. It loads settings,
// builds a doctor snapshot, and reports config, credential resolution, and
// live-probe connectivity for each configured instance.
func RunDoctor(args []string) int {
	// Check for help before flag parsing.
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			printDoctorHelp()
			return 0
		}
	}

	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(doctorStderr)
	verbose := fs.Bool("verbose", false, "print request URLs during checks")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(doctorStderr, "jirafs doctor: invalid flags: %v\n", err)
		return 1
	}
	doctorVerbose = *verbose

	// Load settings.
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(doctorStderr, "jirafs doctor: cannot load settings: %v\n", err)
		return 1
	}

	// Build the doctor snapshot.
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(doctorStderr, "jirafs doctor: cannot determine working directory: %v\n", err)
		return 1
	}
	dsnap := BuildDoctorSnapshot(settings, cwd)

	// Report config (from embedded StatusSnapshot).
	fmt.Fprintln(doctorStdout, "jirafs doctor:")
	fmt.Fprintln(doctorStdout)

	// Project info.
	fmt.Fprintln(doctorStdout, "Config:")
	fmt.Fprintf(doctorStdout, "  project:  %s\n", dsnap.ProjectName)
	fmt.Fprintf(doctorStdout, "  key:      %s\n", dsnap.ProjectKey)
	fmt.Fprintf(doctorStdout, "  instance: %s\n", dsnap.Instance)
	fmt.Fprintf(doctorStdout, "  resolved: %v\n", dsnap.Resolved)
	fmt.Fprintln(doctorStdout)

	// Mirror info.
	if dsnap.MirrorExists {
		fmt.Fprintln(doctorStdout, "Mirror:")
		fmt.Fprintf(doctorStdout, "  dir: %s\n", dsnap.MirrorDir)
		fmt.Fprintf(doctorStdout, "  exists:    %v\n", dsnap.MirrorExists)
		if len(dsnap.MirrorScopes) > 0 {
			fmt.Fprintf(doctorStdout, "  scopes:    %d\n", len(dsnap.MirrorScopes))
			fmt.Fprintf(doctorStdout, "  issues:    %d\n", dsnap.MirrorIssueCount)
			fmt.Fprintf(doctorStdout, "  scope members: %d\n", dsnap.MirrorScopeMemberCount)
		}
		fmt.Fprintln(doctorStdout)
	}

	// Credential checks.
	fmt.Fprintln(doctorStdout, "Credentials:")
	if len(dsnap.InstanceCredentials) == 0 {
		fmt.Fprintln(doctorStdout, "  (no instances configured)")
	} else {
		// Sort instance names for deterministic output.
		instNames := make([]string, 0, len(dsnap.InstanceCredentials))
		for name := range dsnap.InstanceCredentials {
			instNames = append(instNames, name)
		}
		sort.Strings(instNames)
		for _, name := range instNames {
			cc := dsnap.InstanceCredentials[name]
			if cc.Resolved {
				fmt.Fprintf(doctorStdout, "  %s: OK (%s)\n", name, cc.CredentialSummary)
			} else {
				fmt.Fprintf(doctorStdout, "  %s: FAIL — %s\n", name, cc.ValidationError)
			}
		}
	}
	fmt.Fprintln(doctorStdout)

	// Live-probe checks.
	fmt.Fprintln(doctorStdout, "Live Probes:")
	if len(dsnap.LiveProbes) == 0 {
		fmt.Fprintln(doctorStdout, "  (no instances to probe)")
	} else {
		instNames := make([]string, 0, len(dsnap.LiveProbes))
		for name := range dsnap.LiveProbes {
			instNames = append(instNames, name)
		}
		sort.Strings(instNames)
		for _, name := range instNames {
			lpc := dsnap.LiveProbes[name]
			if lpc.Connected {
				if doctorVerbose && lpc.URL != "" {
					fmt.Fprintf(doctorStdout, "  %s: OK (%s, authenticated as %s)\n", name, lpc.URL, lpc.User)
				} else {
					fmt.Fprintf(doctorStdout, "  %s: OK (authenticated as %s)\n", name, lpc.User)
				}
			} else {
				fmt.Fprintf(doctorStdout, "  %s: FAIL — %s\n", name, lpc.Error)
				if lpc.URL != "" {
					fmt.Fprintf(doctorStdout, "    url: %s\n", lpc.URL)
				}
				if lpc.HTTPStatus != 0 {
					fmt.Fprintf(doctorStdout, "    http status: %d\n", lpc.HTTPStatus)
				}
			}
		}
	}
	fmt.Fprintln(doctorStdout)

	// Onboarding / next-step hint (from embedded StatusSnapshot).
	fmt.Fprintln(doctorStdout, "Onboarding:")
	fmt.Fprintf(doctorStdout, "  complete: %v\n", dsnap.OnboardingComplete)
	if len(dsnap.MissingSteps) > 0 {
		fmt.Fprintln(doctorStdout, "  missing steps:")
		for _, step := range dsnap.MissingSteps {
			fmt.Fprintf(doctorStdout, "    - %s\n", step)
		}
		fmt.Fprintf(doctorStdout, "  next step: %s\n", dsnap.NextStep())
		if cmds := onboardingCommands(dsnap); len(cmds) > 0 {
			fmt.Fprintln(doctorStdout, "  useful next commands:")
			for _, cmd := range cmds {
				fmt.Fprintf(doctorStdout, "    - %s\n", cmd)
			}
		}
	} else {
		fmt.Fprintln(doctorStdout, "  next step: (none — all setup complete)")
	}

	return 0
}

// printDoctorHelp prints usage information for the doctor subcommand.
func printDoctorHelp() {
	fmt.Fprintf(doctorStderr, "%s\n", color.BoldBlue(doctorStderr, "Usage:"))
	fmt.Fprintf(doctorStderr, "  jirafs %s [flags]\n\n", color.Blue(doctorStderr, "doctor"))

	fmt.Fprintf(doctorStderr, "%s\n", color.Dim(doctorStderr, "Reports the health of a jirafs workspace: config, credential resolution,"))
	fmt.Fprintf(doctorStderr, "%s\n\n", color.Dim(doctorStderr, "and live-probe connectivity for each configured Jira instance."))

	fmt.Fprintf(doctorStderr, "%s:\n", color.BoldGreen(doctorStderr, "Flags"))
	fmt.Fprintf(doctorStderr, "  %s    %s\n", color.Yellow(doctorStderr, "--verbose"), color.Dim(doctorStderr, "print request URLs during checks"))
	fmt.Fprintf(doctorStderr, "  %s   %s\n", color.Yellow(doctorStderr, "--help, -h"), color.Dim(doctorStderr, "show this help message"))
}

func onboardingCommands(snap DoctorSnapshot) []string {
	for _, step := range snap.MissingSteps {
		if step == "no issues imported or in scope" {
			return []string{
				"jirafs mirror refresh current-sprint",
				"jirafs mirror refresh my-issues",
			}
		}
	}
	return nil
}
