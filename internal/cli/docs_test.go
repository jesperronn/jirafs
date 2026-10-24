package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docFiles are the files that carry generated regions, relative to the repo root.
var docFiles = []string{"README.md", filepath.Join("docs", "cli.md")}

func TestGeneratedDocsUpToDate(t *testing.T) {
	for _, rel := range docFiles {
		path := filepath.Join("..", "..", rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), generatedBeginPrefix) {
			t.Errorf("%s has no generated region", rel)
		}
		got, err := RenderGenerated(string(data))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if got != string(data) {
			t.Errorf("%s is stale: generated regions differ from the command registry; run: bin/gendocs", rel)
		}
	}
}

func TestRenderGeneratedReplacesOnlyRegions(t *testing.T) {
	in := "intro\n<!-- BEGIN GENERATED: commands -->\nold\n<!-- END GENERATED -->\noutro\n"
	out, err := RenderGenerated(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "intro\n<!-- BEGIN GENERATED: commands -->\n| Command | Description |") ||
		!strings.HasSuffix(out, "<!-- END GENERATED -->\noutro\n") || strings.Contains(out, "old") {
		t.Fatalf("unexpected render:\n%s", out)
	}
	again, _ := RenderGenerated(out)
	if again != out {
		t.Fatal("render is not idempotent")
	}
}

func TestRenderGeneratedErrors(t *testing.T) {
	if _, err := RenderGenerated("<!-- BEGIN GENERATED: nope -->\n<!-- END GENERATED -->\n"); err == nil {
		t.Error("unknown region: want error")
	}
	if _, err := RenderGenerated("<!-- BEGIN GENERATED: commands -->\nx\n"); err == nil {
		t.Error("missing END: want error")
	}
}

func TestEveryDispatchedCommandHasRegistryEntry(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "cmd", "jirafs", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	cases := regexp.MustCompile(`case ((?:"[^"]+"(?:, )?)+):`).FindAllStringSubmatch(string(data), -1)
	if len(cases) == 0 {
		t.Fatal("no dispatcher cases found in cmd/jirafs/main.go")
	}
	for _, m := range cases {
		for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(m[1], -1) {
			name := q[1]
			if strings.HasPrefix(name, "-") {
				continue
			}
			if _, ok := LookupCommand(name); !ok {
				t.Errorf("dispatcher command %q has no registry entry", name)
			}
		}
	}
}

func TestRegistryEntriesAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Commands() {
		if seen[c.Name] {
			t.Errorf("duplicate registry entry %q", c.Name)
		}
		seen[c.Name] = true
		if c.Summary == "" || len(c.Usage.Synopsis) == 0 {
			t.Errorf("%q: summary and synopsis are required", c.Name)
		}
		if c.Hidden {
			continue
		}
		var hasHelp bool
		for _, s := range c.Usage.Sections {
			for _, r := range s.Rows {
				if r == helpFlag {
					hasHelp = true
				}
			}
		}
		if !hasHelp && c.Name != "help" && !strings.Contains(strings.Join(c.Usage.Description, ""), "Subcommands") {
			if c.Name != "archive" && c.Name != "registry" {
				t.Errorf("%q: no --help row", c.Name)
			}
		}
	}
}

func TestPrintCommandHelpUsesRegistry(t *testing.T) {
	for _, c := range Commands() {
		var b strings.Builder
		PrintCommandHelp(&b, c.Name)
		if !strings.Contains(b.String(), "Usage:") || !strings.Contains(b.String(), strings.Fields(c.Usage.Synopsis[0])[0]) {
			t.Errorf("%q: help output %q", c.Name, b.String())
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("unknown command: want panic")
		}
	}()
	PrintCommandHelp(&strings.Builder{}, "export")
}

func TestRetiredCommandsAbsentFromDocs(t *testing.T) {
	for _, name := range []string{"sync", "plan", "export", "mirror", "setup"} {
		if _, ok := LookupCommand(name); ok {
			t.Errorf("retired command %q is in the registry", name)
		}
	}
	table := commandsTable()
	if !strings.Contains(table, "`jirafs archive sweep`") {
		t.Errorf("table lacks archive sweep:\n%s", table)
	}
}
