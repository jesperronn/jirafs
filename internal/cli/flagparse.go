package cli

import (
	"flag"
	"os"
)

// parseInterspersed parses args like fs.Parse but allows flags to appear before
// or after positional arguments. A "--" argument ends flag parsing. It returns
// the positionals in order and also leaves them in fs.Args(), so callers that
// use fs.Args()/fs.Arg()/fs.NArg() keep working.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		// fs.Parse strips a "--" terminator; detect it to stop interspersing.
		if i := len(args) - len(rest) - 1; i >= 0 && args[i] == "--" {
			positionals = append(positionals, rest...)
			break
		}
		positionals = append(positionals, rest[0])
		args = rest[1:]
	}
	// Leave all positionals in fs.Args(); flags already parsed stay set.
	if err := fs.Parse(append([]string{"--"}, positionals...)); err != nil {
		return nil, err
	}
	return positionals, nil
}

// parseFlags is parseInterspersed for callers that only need the error.
func parseFlags(fs *flag.FlagSet, args []string) error {
	_, err := parseInterspersed(fs, args)
	return err
}

// scopeFlags holds the --project and --cwd flags shared by commands that
// resolve a configured project. --project is always the name of a project
// from settings.
type scopeFlags struct {
	project *string
	cwd     *string
}

// addScopeFlags registers --project and --cwd on fs.
func addScopeFlags(fs *flag.FlagSet) *scopeFlags {
	return &scopeFlags{
		project: fs.String("project", "", "name of a configured project (default: resolved from --cwd or the remembered project)"),
		cwd:     fs.String("cwd", "", "working directory to resolve the project from (default: current directory)"),
	}
}

// workdir returns the --cwd value, or the process working directory when the
// flag is empty.
func (f *scopeFlags) workdir() (string, error) {
	if *f.cwd != "" {
		return *f.cwd, nil
	}
	return os.Getwd()
}
