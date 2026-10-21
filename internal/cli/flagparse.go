package cli

import "flag"

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
