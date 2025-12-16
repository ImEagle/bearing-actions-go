package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bearing-actions/bearing-actions-go/uml"
)

func main() {
	var (
		includeTests     = flag.Bool("tests", false, "include *_test.go files")
		includeGenerated = flag.Bool("generated", false, "include files with \"Code generated\" headers")
		indent           = flag.String("indent", "  ", "JSON indent (empty for compact)")
		outPath          = flag.String("o", "", "write output to file (default: stdout)")
		exclude          = flag.String("exclude", "", "comma-separated dir names to skip (overrides defaults when set)")
	)
	flag.Parse()

	path := "."
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}

	opts := uml.Options{
		IncludeTests:     *includeTests,
		IncludeGenerated: *includeGenerated,
		Indent:           *indent,
	}
	if *exclude != "" {
		opts.ExcludeDirNames = splitCommaList(*exclude)
	}

	data, err := uml.GenerateJSON(path, opts)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	data = append(data, '\n')

	if *outPath == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "write %s: %v\n", *outPath, err)
		os.Exit(1)
	}
}

func splitCommaList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
