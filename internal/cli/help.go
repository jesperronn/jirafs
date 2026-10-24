package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jirafs/jirafs/internal/color"
)

// Usage describes one command's help page. PrintUsage renders it in the
// shared jirafs style: bold "Usage:" header, coloured sections, dim text.
type Usage struct {
	// Synopsis lines, without the leading "jirafs " (for example
	// "use [flags] [project]"). The first word of each line is the command.
	Synopsis []string
	// Description paragraphs, printed dim.
	Description []string
	// Sections are titled two-column lists (Flags, Subcommands, ...).
	Sections []UsageSection
	// Footer is printed last in cyan; empty means none.
	Footer string
}

// UsageSection is a titled list of name/description rows.
type UsageSection struct {
	Title string
	Rows  [][2]string
}

// PrintUsage writes u to w. Help requested explicitly goes to stdout, so
// callers pass the stdout writer.
func PrintUsage(w io.Writer, u Usage) {
	fmt.Fprintf(w, "%s\n", color.BoldBlue(w, "Usage:"))
	for _, line := range u.Synopsis {
		first, rest, _ := strings.Cut(line, " ")
		out := "  jirafs " + color.Blue(w, first)
		if rest != "" {
			out += " " + rest
		}
		fmt.Fprintln(w, out)
	}
	fmt.Fprintln(w)
	for _, p := range u.Description {
		fmt.Fprintf(w, "%s\n", color.Dim(w, p))
	}
	if len(u.Description) > 0 {
		fmt.Fprintln(w)
	}
	for _, s := range u.Sections {
		width := 0
		for _, r := range s.Rows {
			if len(r[0]) > width {
				width = len(r[0])
			}
		}
		fmt.Fprintf(w, "%s:\n", color.BoldGreen(w, s.Title))
		for _, r := range s.Rows {
			pad := strings.Repeat(" ", width-len(r[0]))
			fmt.Fprintf(w, "  %s%s  %s\n", color.Yellow(w, r[0]), pad, color.Dim(w, r[1]))
		}
		fmt.Fprintln(w)
	}
	if u.Footer != "" {
		fmt.Fprintf(w, "%s\n", color.Cyan(w, u.Footer))
	}
}

// FlagRows converts a FlagSet's flags to Usage rows ("--name VALUE", usage),
// in the lexical order flag.VisitAll uses.
func FlagRows(fs *flag.FlagSet) [][2]string {
	var rows [][2]string
	fs.VisitAll(func(f *flag.Flag) {
		name, usage := flag.UnquoteUsage(f)
		left := "--" + f.Name
		if name != "" {
			left += " " + name
		}
		rows = append(rows, [2]string{left, usage})
	})
	return rows
}

// isHelpArg reports whether a is an explicit help request.
func isHelpArg(a string) bool {
	return a == "--help" || a == "-h" || a == "-help"
}

// wantsHelp reports whether args contain an explicit help flag before any
// "--" terminator.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if isHelpArg(a) {
			return true
		}
	}
	return false
}

// wantsHelpOrWord is wantsHelp, additionally accepting the bare word "help"
// as the first argument.
func wantsHelpOrWord(args []string) bool {
	return (len(args) > 0 && args[0] == "help") || wantsHelp(args)
}

// usageError prints one error line plus the standard hint to w and returns
// the usage-error exit code 2. cmd is the command prefix, e.g. "jirafs sync".
func usageError(w io.Writer, cmd, format string, a ...any) int {
	fmt.Fprintf(w, "%s: %s\n", cmd, fmt.Sprintf(format, a...))
	fmt.Fprintf(w, "Use --help for usage.\n")
	return 2
}

// flagError reports a flag parse failure as a usage error.
func flagError(w io.Writer, cmd string, err error) int {
	msg := err.Error()
	if errors.Is(err, flag.ErrHelp) {
		msg = "unknown help flag"
	}
	return usageError(w, cmd, "%s", msg)
}

// quietFlagSet returns a FlagSet that never prints on its own; callers report
// parse errors through flagError.
func quietFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	return fs
}
