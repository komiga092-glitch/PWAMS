package main

import (
	"html/template"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/komiga092-glitch/pwams/internal/routes"
)

// templateRef matches `{{ template "name" ... }}` and `{{ block "name" ... }}`
// references. html/template only fails on an unresolvable reference at EXECUTE
// time, which is exactly how a partially-loaded set produced HTTP 200 with a
// zero-byte body, so the check has to be done statically here.
var templateRef = regexp.MustCompile(`\{\{-?\s*(?:template|block)\s+"([^"]+)"`)

// tplcheck parses the authoritative PWAMS template set (the exact files
// cmd/server loads), fails if any file has a syntax error or an unknown
// function, and fails if any template references a template that the set does
// not define.
func main() {
	files := routes.TemplateFiles()
	funcs := template.FuncMap(routes.TemplateFuncMap())
	ok := true

	for _, f := range files {
		t := template.New("check").Funcs(funcs)
		if _, err := t.ParseFiles(f); err != nil {
			println("PARSE ERROR:", f, "=>", err.Error())
			ok = false
		}
	}

	// Parse the whole set together, then enumerate the defined template names
	// and validate every cross-reference against them.
	set, err := template.New("check-all").Funcs(funcs).ParseFiles(files...)
	if err != nil {
		println("PARSE ERROR (full set):", err.Error())
		os.Exit(1)
	}

	defined := map[string]bool{}

	for _, t := range set.Templates() {
		defined[t.Name()] = true
	}

	referenced := map[string][]string{}

	for _, file := range files {
		raw, readErr := os.ReadFile(file)
		if readErr != nil {
			println("READ ERROR:", file, "=>", readErr.Error())
			ok = false
			continue
		}

		for _, match := range templateRef.FindAllStringSubmatch(string(raw), -1) {
			name := match[1]
			referenced[name] = append(referenced[name], file)
		}
	}

	names := make([]string, 0, len(referenced))

	for name := range referenced {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		if defined[name] {
			continue
		}

		ok = false
		println(
			"UNDEFINED TEMPLATE REFERENCE:",
			name,
			"=> referenced by",
			strings.Join(referenced[name], ", "),
		)
	}

	if !ok {
		os.Exit(1)
	}

	println("ALL TEMPLATES PARSED OK, TEMPLATE COUNT:", len(defined))
}
