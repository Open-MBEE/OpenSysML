package codes

import (
	"bytes"
	"fmt"
	"strings"
)

// Markers delimit the generated catalogue in docs/reference/diagnostics.md:
// what lies between them is written by Catalogue and checked against it.
const (
	BeginMarker = "<!-- diagnostics-catalogue:begin -->"
	EndMarker   = "<!-- diagnostics-catalogue:end -->"
)

// Catalogue renders every registered code as Markdown, grouped by tier in
// catalogue order, each under a heading whose anchor is the code itself, so
// `sysml -explain <code>`, the editor's code description and this page agree.
func Catalogue() string {
	var b strings.Builder
	b.WriteString("_Generated from the diagnostic code registry by `make diagnostics-catalogue`; `make diagnostics-check` fails when this section drifts from it. Edit the registry, not this text._\n")
	for _, t := range Tiers() {
		entries := ByTier(t)
		if len(entries) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### %s\n\n", tierTitles[t])
		fmt.Fprintf(&b, "| Code | Severity | Summary |\n|------|----------|---------|\n")
		for _, e := range entries {
			fmt.Fprintf(&b, "| [`%s`](#%s) | %s | %s |\n", e.Code, Anchor(e.Code), e.Severity, e.Summary)
		}
		for _, e := range entries {
			fmt.Fprintf(&b, "\n#### `%s`\n\n", e.Code)
			fmt.Fprintf(&b, "**%s** · %s · %s\n\n", e.Severity, tierTitles[t], e.Summary)
			b.WriteString(strings.TrimRight(e.Meaning, "\n"))
			b.WriteString("\n")
			if e.IsFamily() {
				fmt.Fprintf(&b, "\nEmitted as `%s-<rule>`, one code per rule: ", e.Code)
				for i, k := range e.Kinds {
					if i > 0 {
						b.WriteString(", ")
					}
					fmt.Fprintf(&b, "`%s-%s`", e.Code, k)
					if severity := e.MemberSeverity(k); severity != e.Severity {
						fmt.Fprintf(&b, " (a %s)", severity)
					}
				}
				b.WriteString(".\n")
			}
			if e.Spec != "" {
				fmt.Fprintf(&b, "\nSpecification: %s\n", e.Spec)
			}
		}
	}
	return b.String()
}

// Splice returns page with the text between the markers replaced by the
// catalogue, or an error when the page lacks a marker.
func Splice(page []byte) ([]byte, error) {
	begin := bytes.Index(page, []byte(BeginMarker))
	end := bytes.Index(page, []byte(EndMarker))
	if begin < 0 || end < 0 || end < begin {
		return nil, fmt.Errorf("the page lacks the %s … %s markers", BeginMarker, EndMarker)
	}
	var out bytes.Buffer
	out.Write(page[:begin+len(BeginMarker)])
	out.WriteString("\n")
	out.WriteString(Catalogue())
	out.Write(page[end:])
	return out.Bytes(), nil
}

// Anchor is the heading anchor of a code's catalogue section, as the site's
// slugger makes it: lower case, letters, digits, dashes and underscores kept,
// so `runtime.materialize` is `runtimematerialize`.
func Anchor(code string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 'a' - 'A'
		}
		return -1
	}, code)
}
