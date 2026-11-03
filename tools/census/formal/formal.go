// Package formal reads the check*, derive* and validate* constraints of the
// pinned KerML and SysML documents (the editions the manifest pins by SHA-256) from the Constraints
// subsections of their abstract-syntax clauses and records the ones no pilot
// validator names as spec-sourced rows of the constraint manifest.
//
// The documents are the PDFs the manifest pins by SHA-256, read with
// `pdftotext -layout`. -list prints the extracted constraints, -update
// records them in the manifest, keeping every hand-edited field, and -check
// verifies the manifest is what -update would write. Run it with
// `go run -C tools ./cmd/formal-constraints`.
package formal

import (
	"fmt"
	"regexp"
	"strings"
)

// Constraint is one constraint as a formal document states it.
type Constraint struct {
	// Document is the manifest's key of the specification stating it.
	Document string `json:"document"`
	// Clause is the abstract-syntax clause whose Constraints subsection states it.
	Clause    string `json:"clause"`
	Metaclass string `json:"metaclass"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	// Checks is the normative prose, one line; the OCL when there is no prose.
	Checks string `json:"checks"`
	// OCL is the constraint's OCL verbatim, empty when the document states none.
	OCL string `json:"ocl,omitempty"`
}

// Method states how the constraints are read, for the manifest's record.
const Method = "pdftotext -layout over each pinned PDF; within clause 8.3 (abstract syntax), each `8.3.x…` heading opens a metaclass and the Constraints subsection's bare check*/derive*/validate* lines name its constraints; the leading sentence paragraphs are the prose, the remaining paragraphs the OCL"

var (
	footerPattern  = regexp.MustCompile(`^\s*(\d+\s+)?(Kernel Modeling Language v\d+\.\d+(?:, Beta \d+)?|Systems Modeling Language v\d+\.\d+(?:, Beta \d+)?, Part 1)(\s+\d+)?\s*$`)
	clauseStart    = regexp.MustCompile(`^8\.3\s+\S`)
	clauseEnd      = regexp.MustCompile(`^8\.4\s+\S`)
	headingPattern = regexp.MustCompile(`^(8\.3(?:\.\d+)+)\s+(\S.*?)\s*$`)
	namePattern    = regexp.MustCompile(`^(check|derive|validate)[A-Z][A-Za-z0-9]*$`)
	oclPattern     = regexp.MustCompile(`->|<>|oclIs|specializesFromLibrary\(|^(let|if|not)\b|(^|\s)--\s`)
	oclStart       = regexp.MustCompile(`^[a-z]|->|<>|oclIs|specializesFromLibrary\(`)
	proseStart     = regexp.MustCompile(`^[A-Z"“•]`)
	prosePattern   = regexp.MustCompile(`^[A-Z"“•]|\.$`)
	spaces         = regexp.MustCompile(`\s+`)
)

// subsections are the headings of a metaclass clause; the constraints follow
// the last one.
var subsections = map[string]bool{"Description": true, "General Classes": true, "Attributes": true, "Operations": true, "Constraints": true}

// Parse reads the constraints of one document from its `pdftotext -layout`
// text, in document order. Document is left for the caller to set.
func Parse(text string) ([]Constraint, error) {
	lines := clean(text)
	start, end, err := abstractSyntax(lines)
	if err != nil {
		return nil, err
	}
	var (
		out       []Constraint
		clause    string
		metaclass string
		section   string
		current   *Constraint
		body      []string
	)
	flush := func() {
		if current != nil {
			current.Checks, current.OCL = split(body)
			out = append(out, *current)
		}
		current, body = nil, nil
	}
	for _, line := range lines[start:end] {
		if m := headingPattern.FindStringSubmatch(line); m != nil {
			flush()
			clause, metaclass, section = m[1], strings.ReplaceAll(m[2], " ", ""), ""
			continue
		}
		if subsections[line] {
			flush()
			section = line
			continue
		}
		if section != "Constraints" {
			continue
		}
		if namePattern.MatchString(line) {
			flush()
			current = &Constraint{Clause: clause, Metaclass: metaclass, Kind: namePattern.FindStringSubmatch(line)[1], Name: line}
			continue
		}
		if current != nil {
			body = append(body, line)
		}
	}
	flush()
	seen := make(map[string]bool, len(out))
	for _, c := range out {
		if seen[c.Name] {
			return nil, fmt.Errorf("%s is stated twice", c.Name)
		}
		seen[c.Name] = true
	}
	return out, nil
}

// clean drops the page breaks and running footers and trailing blanks, so a
// constraint spanning a page reads as one.
func clean(text string) []string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\f", ""), "\n") {
		if footerPattern.MatchString(line) {
			continue
		}
		lines = append(lines, strings.TrimRight(line, " \t\r"))
	}
	return lines
}

// abstractSyntax locates clause 8.3 in the body, past the table of contents.
func abstractSyntax(lines []string) (int, int, error) {
	start := -1
	for i, line := range lines {
		if clauseStart.MatchString(line) && !strings.Contains(line, "....") {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, 0, fmt.Errorf("clause 8.3 not found")
	}
	for i := start + 1; i < len(lines); i++ {
		if clauseEnd.MatchString(lines[i]) {
			return start, i, nil
		}
	}
	return 0, 0, fmt.Errorf("clause 8.4 not found after 8.3")
}

// split separates a constraint's body into its prose and its OCL: the leading
// paragraphs that read as sentences are the prose, the rest the OCL. A body
// with no prose states its OCL as the prose too, so every row has text.
func split(body []string) (string, string) {
	paragraphs := paragraphsOf(body)
	var prose, ocl []string
	for i, p := range paragraphs {
		if len(ocl) == 0 && isProse(p) {
			prose = append(prose, flatten(p))
			continue
		}
		ocl = append(ocl, strings.Join(paragraphs[i], "\n"))
	}
	checks := strings.Join(prose, " ")
	oclText := strings.Join(ocl, "\n\n")
	if checks == "" {
		checks = flatten(strings.Split(oclText, "\n"))
	}
	return checks, oclText
}

// paragraphsOf groups the body's lines at blank lines, and also where a
// sentence ends and OCL begins on the very next line, as the documents set a
// few constraints. A line stating only "TBD" is the document's placeholder for
// OCL it does not give, and is kept with the OCL.
func paragraphsOf(body []string) [][]string {
	var paragraphs [][]string
	var paragraph []string
	flush := func() {
		if len(paragraph) > 0 {
			paragraphs = append(paragraphs, paragraph)
			paragraph = nil
		}
	}
	for _, line := range body {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if n := len(paragraph); n > 0 && strings.HasSuffix(paragraph[n-1], ".") && oclStart.MatchString(line) && proseStarts(paragraph) && isProse(paragraph) {
			flush()
		}
		paragraph = append(paragraph, line)
	}
	flush()
	return paragraphs
}

func isProse(paragraph []string) bool {
	text := flatten(paragraph)
	return text != "TBD" && prosePattern.MatchString(text) && !oclPattern.MatchString(text)
}

// proseStarts reports whether a paragraph opens as a sentence does.
func proseStarts(paragraph []string) bool {
	return len(paragraph) > 0 && proseStart.MatchString(strings.TrimSpace(paragraph[0]))
}

// flatten joins wrapped lines into one, with single spaces.
func flatten(lines []string) string {
	return strings.TrimSpace(spaces.ReplaceAllString(strings.Join(lines, " "), " "))
}
