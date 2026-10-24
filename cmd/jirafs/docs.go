package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jirafs/jirafs/internal/cli"
)

// runDocs implements the hidden `jirafs docs` command used by bin/gendocs.
//
//	docs --markdown      print the command table
//	docs --render FILE   print FILE with its generated regions refreshed
func runDocs(args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 1 && args[0] == "--markdown":
		table, err := cli.GeneratedRegion("commands")
		if err != nil {
			fmt.Fprintf(stderr, "jirafs docs: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, table)
		return 0
	case len(args) == 2 && args[0] == "--render":
		data, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "jirafs docs: %v\n", err)
			return 1
		}
		out, err := cli.RenderGenerated(string(data))
		if err != nil {
			fmt.Fprintf(stderr, "jirafs docs: %s: %v\n", args[1], err)
			return 1
		}
		fmt.Fprint(stdout, out)
		return 0
	}
	fmt.Fprintf(stderr, "jirafs docs: usage: jirafs docs --markdown | --render FILE\n")
	return 2
}
