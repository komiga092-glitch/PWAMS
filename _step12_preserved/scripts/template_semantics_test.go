package main

import (
	"fmt"
	"html/template"
	"strings"
	"testing"
)

// TestElseIfSemantics verifies how Go's html/template parser treats
// {{ if }}{{ else if }} chains and how many {{ end }} tags are needed.
//
// html/template rewrites {{ else if X }} into {{ else }}{{ if X }}, so an
// entire if/else-if chain is closed by a single {{ end }}. Each control
// construct therefore needs exactly one {{ end }} (plus one for any
// enclosing {{ define }} block); extra {{ end }} tags are parse errors.
// The nested pattern below is the reports.html pagination structure whose
// missing {{ end }} broke template parsing.
func TestElseIfSemantics(t *testing.T) {
	// Correctly balanced chain: one {{ end }} closes the if/else-if chain,
	// one closes the {{ define }} block.
	balanced := `{{ define "t1" }}{{ if .A }}A{{ else if .B }}B{{ end }}{{ end }}`
	if _, err := template.New("t1").Parse(balanced); err != nil {
		t.Fatalf("expected balanced chain to parse, got: %v", err)
	}

	// One {{ end }} too many: both the chain and the define are already
	// closed, so the third {{ end }} is unexpected.
	extraEnd := `{{ define "t2" }}{{ if .A }}A{{ else if .B }}B{{ end }}{{ end }}{{ end }}`
	if err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		_, err = template.New("t2").Parse(extraEnd)
		return err
	}(); err == nil || !strings.Contains(err.Error(), "unexpected {{end}}") {
		t.Fatalf("expected extra end to fail with \"unexpected {{end}}\", got: %v", err)
	}

	// Now replicate the exact broken pattern seen in reports.html:
	// nested if + with per branch, only ONE end per branch.
	broken := `{{ define "rp" }}{{ if .error }}{{ else if .rt }}{{ if eq .rt "u" }}{{ with .r }}u{{ end }}{{ else if eq .rt "b" }}{{ with .r }}b{{ end }}{{ end }}{{ end }}{{ define "rp2" }}x{{ end }}`
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		_, err = template.New("rp").Parse(broken)
		return err
	}()
	if err == nil || !strings.Contains(err.Error(), "unexpected <define> in command") {
		t.Fatalf("expected broken pattern to fail like reports.html, got: %v", err)
	}

	// The FIXED pattern: one {{ end }} closes the inner if/else-if chain,
	// one closes the outer if/else-if chain, and one closes the
	// {{ define }} block.
	fixed := `{{ define "rp" }}{{ if .error }}{{ else if .rt }}{{ if eq .rt "u" }}{{ with .r }}u{{ end }}{{ else if eq .rt "b" }}{{ with .r }}b{{ end }}{{ end }}{{ end }}{{ end }}{{ define "rp2" }}x{{ end }}`
	if err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		_, err = template.New("rp").Parse(fixed)
		return err
	}(); err != nil {
		t.Fatalf("expected fixed pattern to parse, got: %v", err)
	}
}
