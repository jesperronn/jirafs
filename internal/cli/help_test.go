package cli

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestPrintUsage(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	fs.String("name", "", "the name")
	fs.Bool("fast", false, "go fast")
	rows := FlagRows(fs)
	if len(rows) != 2 || rows[0][0] != "--fast" || rows[1][0] != "--name string" || rows[1][1] != "the name" {
		t.Fatalf("FlagRows = %v", rows)
	}
	var b bytes.Buffer
	PrintUsage(&b, Usage{
		Synopsis:    []string{"x [flags]", "x"},
		Description: []string{"Does x."},
		Sections:    []UsageSection{{Title: "Flags", Rows: rows}},
		Footer:      "Run it.",
	})
	out := b.String()
	for _, want := range []string{"Usage:\n  jirafs x [flags]\n  jirafs x\n", "Does x.", "Flags:\n", "--fast         go fast", "Run it."} {
		if !strings.Contains(out, want) {
			t.Errorf("usage missing %q:\n%s", want, out)
		}
	}
	b.Reset()
	PrintUsage(&b, Usage{Synopsis: []string{"y"}})
	if strings.Contains(b.String(), "Run it.") {
		t.Errorf("empty footer printed: %q", b.String())
	}
}

func TestWantsHelp(t *testing.T) {
	for _, c := range []struct {
		args       []string
		help, word bool
	}{
		{[]string{"--help"}, true, true},
		{[]string{"a", "-h"}, true, true},
		{[]string{"-help"}, true, true},
		{[]string{"help"}, false, true},
		{[]string{"a", "help"}, false, false},
		{[]string{"--", "--help"}, false, false},
		{nil, false, false},
	} {
		if got := wantsHelp(c.args); got != c.help {
			t.Errorf("wantsHelp(%v) = %v", c.args, got)
		}
		if got := wantsHelpOrWord(c.args); got != c.word {
			t.Errorf("wantsHelpOrWord(%v) = %v", c.args, got)
		}
	}
}

func TestFlagError(t *testing.T) {
	var b bytes.Buffer
	if code := flagError(&b, "jirafs x", errors.New("bad")); code != 2 {
		t.Fatalf("code = %d", code)
	}
	if got := b.String(); got != "jirafs x: bad\nUse --help for usage.\n" {
		t.Fatalf("output = %q", got)
	}
	b.Reset()
	flagError(&b, "jirafs x", flag.ErrHelp)
	if !strings.Contains(b.String(), "unknown help flag") {
		t.Fatalf("output = %q", b.String())
	}
}

// TestSubcommandHelpBeforeSettings runs every subcommand help with no
// settings file; help must print on stdout, exit 0 and leave stderr empty.
func TestSubcommandHelpBeforeSettings(t *testing.T) {
	archOut, archErr := withArchiveTestIO(t)
	expOut, expErr := withExportTestIO(t)
	mirOut, mirErr := withMirrorTestIO(t)
	cases := []struct {
		name     string
		run      func() int
		out, err *bytes.Buffer
	}{
		{"archive issue", func() int { return RunArchive([]string{"issue", "--help"}) }, archOut, archErr},
		{"archive list", func() int { return RunArchive([]string{"list", "-h"}) }, archOut, archErr},
		{"export issue", func() int { return RunExport([]string{"issue", "--help"}) }, expOut, expErr},
		{"mirror refresh", func() int { return RunMirror([]string{"refresh", "--help"}) }, mirOut, mirErr},
		{"mirror my-issues", func() int { return RunMirror([]string{"my-issues", "-h"}) }, mirOut, mirErr},
		{"mirror archive-sweep", func() int { return RunMirror([]string{"archive-sweep", "--help"}) }, mirOut, mirErr},
	}
	for _, c := range cases {
		c.out.Reset()
		c.err.Reset()
		if code := c.run(); code != 0 {
			t.Errorf("%s: exit = %d, want 0", c.name, code)
		}
		if !strings.Contains(c.out.String(), "Usage:") || c.err.Len() != 0 {
			t.Errorf("%s: stdout = %q stderr = %q", c.name, c.out.String(), c.err.String())
		}
	}
}
