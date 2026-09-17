package main

// i18nrepl is a one-shot migration helper: it applies literal text
// replacements listed in a JSON file of the form
//
//	{ "path/to/file.html": [["old text", "new text"], ...], ... }
//
// It prints "MISS <file>: <snippet>" for any pattern that did not match so
// leftovers can be fixed by hand. Used to thread {{t .Lang "..."}} calls
// through the templates.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: i18nrepl <mapping.json>")
		os.Exit(1)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "read mapping:", err)
		os.Exit(1)
	}
	var mapping map[string][][]string
	if err := json.Unmarshal(data, &mapping); err != nil {
		fmt.Fprintln(os.Stderr, "parse mapping:", err)
		os.Exit(1)
	}
	misses := 0
	for path, pairs := range mapping {
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read file:", err)
			os.Exit(1)
		}
		content := string(raw)
		for _, pair := range pairs {
			if len(pair) != 2 {
				fmt.Fprintf(os.Stderr, "bad pair for %s\n", path)
				os.Exit(1)
			}
			old, repl := pair[0], pair[1]
			if !strings.Contains(content, old) && strings.Contains(old, "\n") &&
				!strings.Contains(old, "\r\n") && strings.Contains(content, "\r\n") {
				// CRLF file: retry the pattern with \r\n line endings.
				old = strings.ReplaceAll(old, "\n", "\r\n")
				repl = strings.ReplaceAll(repl, "\n", "\r\n")
			}
			if !strings.Contains(content, old) {
				snippet := pair[0]
				if len(snippet) > 50 {
					snippet = snippet[:50]
				}
				fmt.Printf("MISS %s: %q\n", path, snippet)
				misses++
				continue
			}
			content = strings.ReplaceAll(content, old, repl)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "write file:", err)
			os.Exit(1)
		}
	}
	fmt.Printf("done, %d misses\n", misses)
	if misses > 0 {
		os.Exit(2)
	}
}
