package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/report/junit"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/report/sarif"
)

// The report formats -format names; -json is -format json.
const (
	formatText  = "text"
	formatJSON  = "json"
	formatSARIF = "sarif"
	formatJUnit = "junit"
)

// fallbackRuleID files a diagnostic that carries no code and names no pass.
const fallbackRuleID = "diagnostic"

// reportFormat resolves -format and -json into one format, or says why not.
func reportFormat(format string, asJSON bool) (string, string) {
	switch {
	case format == "" && asJSON:
		return formatJSON, ""
	case format == "":
		return formatText, ""
	case asJSON && format != formatJSON:
		return "", fmt.Sprintf("-json reports JSON; it cannot be combined with -format %s", format)
	}
	switch format {
	case formatText, formatJSON, formatSARIF, formatJUnit:
		return format, ""
	}
	return "", fmt.Sprintf("-format %s is not a report format; the formats are text, json, sarif and junit", format)
}

// workspacePath spells a model path relative to the workspace root, the working
// directory, with forward slashes, so a report written on one machine names
// the same file on another; a path outside the workspace stays absolute.
func workspacePath(root, path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// baseline is the set of known findings a -baseline file accepts: each is
// matched on its code, its workspace-relative file and its message, so a
// finding that only moved to another line stays known.
type baseline map[baselineKey]bool

type baselineKey struct{ code, file, message string }

// readBaseline reads the diagnostics of a JSON report, as -validate -json -o
// wrote it, into the set of findings it accepts.
func readBaseline(root, path string) (baseline, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the baseline is the file the command line names
	if err != nil {
		return nil, err
	}
	var recorded struct {
		Diagnostics []diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(data, &recorded); err != nil {
		return nil, fmt.Errorf("%s: not a JSON report: %v", path, err)
	}
	known := make(baseline, len(recorded.Diagnostics))
	for _, d := range recorded.Diagnostics {
		known[baselineKey{d.Code, workspacePath(root, d.File), d.Message}] = true
	}
	return known, nil
}

// accepts reports whether the baseline knows a finding of file.
func (b baseline) accepts(root string) func(file, code, message string) bool {
	return func(file, code, message string) bool {
		return b[baselineKey{code, workspacePath(root, file), message}]
	}
}

// ruleID is the SARIF rule a diagnostic is filed under: its code, else
// fallbackRuleID, the one rule for a finding that carries none.
func ruleID(d diagnostic) string {
	if d.Code != "" {
		return d.Code
	}
	return fallbackRuleID
}

// sarifReport renders the diagnostics as one SARIF run of this build, one rule
// per code, each result on its file relative to the workspace root.
func (r *reporter) sarifReport() *sarif.Log {
	log := sarif.NewLog(sarif.Driver{
		Name:           "sysml",
		InformationURI: "https://github.com/Open-MBEE/OpenSysML",
		Version:        Version,
	})
	run := &log.Runs[0]
	run.ColumnKind = "utf16CodeUnits"
	rules := map[string]int{}
	for _, d := range r.report.Diagnostics {
		id := ruleID(d)
		index, known := rules[id]
		if !known {
			index = len(run.Tool.Driver.Rules)
			rules[id] = index
			run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, sarif.Rule{
				ID:               id,
				ShortDescription: sarif.Message{Text: ruleDescription(d, id)},
			})
		}
		uri := workspacePath(r.root, d.File)
		result := sarif.Result{
			RuleID:    id,
			RuleIndex: index,
			Level:     sarif.Level(d.Severity),
			Message:   sarif.Message{Text: d.Message},
		}
		if d.File != "" {
			result.Locations = sarif.LocationAt(uri, d.Line, d.utf16Column)
		}
		for _, f := range d.Fixes {
			result.Fixes = append(result.Fixes, sarifFix(uri, f))
		}
		run.Results = append(run.Results, result)
	}
	sarif.SortResults(run.Results)
	if r.suppressed > 0 {
		run.Properties = &sarif.Properties{Suppressed: r.suppressed}
	}
	return log
}

// ruleDescription describes a rule by its id, which names what it finds.
func ruleDescription(d diagnostic, id string) string {
	if d.Code != "" {
		return "A finding of kind " + id + "."
	}
	return "A finding that carries no code."
}

func sarifFix(uri string, f fix) sarif.Fix {
	change := sarif.ArtifactChange{ArtifactLocation: sarif.ArtifactLocation{URI: uri}}
	for _, e := range f.Edits {
		replacement := sarif.Replacement{DeletedRegion: sarif.Region{
			StartLine: e.Line, StartColumn: e.utf16Column, EndLine: e.EndLine, EndColumn: e.utf16EndColumn,
		}}
		if e.NewText != "" {
			replacement.InsertedContent = &sarif.ArtifactContent{Text: e.NewText}
		}
		change.Replacements = append(change.Replacements, replacement)
	}
	return sarif.Fix{Description: sarif.Message{Text: f.Title}, ArtifactChanges: []sarif.ArtifactChange{change}}
}

// junitReport renders the diagnostics as JUnit XML: one suite per model file,
// one failing case per finding and one passing case for a file with none; what
// stopped the run is an error case of the run's own suite, and a file with no
// finding in a run stopped by something no finding explains is skipped.
func (r *reporter) junitReport() *junit.Testsuites {
	byFile := map[string][]diagnostic{}
	for _, file := range r.files {
		byFile[workspacePath(r.root, file)] = nil
	}
	if _, present := byFile[""]; !present && len(r.report.Errors) > 0 {
		byFile[""] = nil
	}
	unexplained := len(r.report.Errors) > 0 && !r.hasErrorDiagnostics()
	for _, d := range r.report.Diagnostics {
		key := workspacePath(r.root, d.File)
		byFile[key] = append(byFile[key], d)
	}
	names := make([]string, 0, len(byFile))
	for name := range byFile {
		names = append(names, name)
	}
	sort.Strings(names)
	suites := &junit.Testsuites{Name: "sysml -validate"}
	for _, name := range names {
		suite := junit.Testsuite{Name: name}
		if name == "" {
			// Findings of the run itself, which no model file locates.
			suite.Name = "sysml"
		}
		found := byFile[name]
		if name == "" {
			for _, message := range r.report.Errors {
				suite.AddCase(junit.Testcase{Name: "run", Classname: suite.Name, Error: &junit.Message{Text: message, Type: "error", Body: message}})
			}
		} else if len(found) == 0 {
			c := junit.Testcase{Name: "no findings", Classname: name, File: name}
			if unexplained {
				c.Skipped = &junit.Message{Text: "validation did not complete: " + r.report.Errors[0]}
			}
			suite.AddCase(c)
		}
		for _, d := range found {
			c := junit.Testcase{Name: junitCaseName(d), Classname: suite.Name, File: name}
			body := d.Message
			if d.File != "" {
				body = fmt.Sprintf("%s:%d:%d: %s: %s", name, d.Line, d.Column, d.Severity, d.Message)
			}
			// A finding about the model fails its file's case; one about the run
			// itself, which no file locates, is an error of the run.
			message := &junit.Message{Text: d.Message, Type: d.Severity, Body: body}
			if d.File == "" {
				c.Error = message
			} else {
				c.Failure = message
			}
			suite.AddCase(c)
		}
		suites.AddSuite(suite)
	}
	return suites
}

// hasErrorDiagnostics reports whether a finding of severity error was recorded.
func (r *reporter) hasErrorDiagnostics() bool {
	for _, d := range r.report.Diagnostics {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

func junitCaseName(d diagnostic) string {
	name := ruleID(d)
	if d.File != "" {
		name = fmt.Sprintf("%s at %d:%d", name, d.Line, d.Column)
	}
	return name
}
