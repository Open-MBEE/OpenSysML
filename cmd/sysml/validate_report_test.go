package main

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/report/junit"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/report/sarif"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/report/sarif/sariftest"
)

const (
	reportCleanModel  = "package Q {\n  part def B;\n}\n"
	reportBrokenModel = "package P {\n  part def A;\n  part a : A;\n  part b : Missing;\n}\n"
	// reportNotationModel analyses cleanly with one warning, which carries a fix.
	reportNotationModel = "package S {\n  state def M {\n    entry; then pick;\n    choice pick;\n    transition pick then a;\n    state a;\n  }\n}\n"
)

// reportWorkspace is a workspace of three model files under models/: one
// clean, one with an error and one with a warning that has a fix.
func reportWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"clean.sysml": reportCleanModel, "broken.sysml": reportBrokenModel, "notation.sysml": reportNotationModel} {
		if err := os.WriteFile(filepath.Join(dir, "models", name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// severityLine matches a diagnostic line's severity marker, with or without
// the code a line may carry after it: `warning:` or `warning[some-code]:`.
func severityLine(severity string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)\b` + regexp.QuoteMeta(severity) + `(\[[^\]]*\])?:`)
}

func wantSeverity(t *testing.T, got runOutcome, severity string) {
	t.Helper()
	if !severityLine(severity).MatchString(got.output()) {
		t.Errorf("report has no %s line:\n%s", severity, got.output())
	}
}

func rejectSeverity(t *testing.T, got runOutcome, severity string) {
	t.Helper()
	if severityLine(severity).MatchString(got.output()) {
		t.Errorf("report has a %s line:\n%s", severity, got.output())
	}
}

func decodeSARIF(t *testing.T, data []byte) sarif.Log {
	t.Helper()
	if err := sariftest.Validate(data); err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	var log sarif.Log
	if err := json.Unmarshal(data, &log); err != nil {
		t.Fatal(err)
	}
	return log
}

// TestValidateSARIFReport: -format sarif writes one SARIF 2.1.0 run of this
// build, one rule per code, each result on its workspace-relative file with
// its line and column, and the fixes analysis knows.
func TestValidateSARIFReport(t *testing.T) {
	binary := buildCLI(t)
	dir := reportWorkspace(t)
	got := checkPathsIn(t, dir, binary, "-validate", "-format", "sarif", "-o", "report.sarif", "models")
	if got.status != exitUnevaluable || got.stdout != "" {
		t.Fatalf("exit %d, stdout %q; want %d and the report in the file\n%s", got.status, got.stdout, exitUnevaluable, got.output())
	}
	data, err := os.ReadFile(filepath.Join(dir, "report.sarif"))
	if err != nil {
		t.Fatal(err)
	}
	log := decodeSARIF(t, data)
	if len(log.Runs) != 1 || log.Runs[0].Tool.Driver.Name != "sysml" || log.Runs[0].Tool.Driver.Version != Version {
		t.Fatalf("run = %+v", log.Runs)
	}
	run := log.Runs[0]
	if run.ColumnKind != "utf16CodeUnits" {
		t.Fatalf("columnKind = %q", run.ColumnKind)
	}
	rules := map[string]int{}
	for i, rule := range run.Tool.Driver.Rules {
		if _, dup := rules[rule.ID]; dup {
			t.Fatalf("rule %q listed twice", rule.ID)
		}
		rules[rule.ID] = i
	}
	byRule := map[string]sarif.Result{}
	for _, r := range run.Results {
		if rules[r.RuleID] != r.RuleIndex {
			t.Fatalf("result %q has ruleIndex %d, the rule is at %d", r.RuleID, r.RuleIndex, rules[r.RuleID])
		}
		byRule[r.RuleID] = r
	}
	unresolved, ok := byRule["unresolved"]
	if !ok || unresolved.Level != "error" {
		t.Fatalf("unresolved reference not reported as an error: %+v", run.Results)
	}
	loc := unresolved.Locations[0].PhysicalLocation
	if loc.ArtifactLocation.URI != "models/broken.sysml" || loc.Region.StartLine != 4 || loc.Region.StartColumn != 12 {
		t.Fatalf("unresolved reference placed at %+v", loc)
	}
	notation, ok := byRule["nonstandard-notation"]
	if !ok || notation.Level != "warning" {
		t.Fatalf("notation warning not reported with its own level: %+v", run.Results)
	}
	if len(notation.Fixes) != 1 || len(notation.Fixes[0].ArtifactChanges) != 1 {
		t.Fatalf("notation fix = %+v", notation.Fixes)
	}
	change := notation.Fixes[0].ArtifactChanges[0]
	if change.ArtifactLocation.URI != "models/notation.sysml" || len(change.Replacements) == 0 ||
		change.Replacements[0].InsertedContent == nil || change.Replacements[0].InsertedContent.Text != "#choice state pick;" {
		t.Fatalf("notation fix change = %+v", change)
	}
	if change.Replacements[0].DeletedRegion.StartLine != 4 || change.Replacements[0].DeletedRegion.EndColumn <= change.Replacements[0].DeletedRegion.StartColumn {
		t.Fatalf("deleted region = %+v", change.Replacements[0].DeletedRegion)
	}
}

// TestValidateSARIFOnExamples: the showcase examples report as valid SARIF to
// stdout, and -format json is -json.
func TestValidateSARIFOnExamples(t *testing.T) {
	binary := buildCLI(t)
	got := checkPaths(t, binary, "-validate", "-format", "sarif", "../../examples/runtime-showcase")
	log := decodeSARIF(t, []byte(got.stdout))
	if len(log.Runs) != 1 {
		t.Fatalf("runs = %d", len(log.Runs))
	}
	asJSON := checkPaths(t, binary, "-validate", "-json", "../../examples/runtime-showcase")
	asFormat := checkPaths(t, binary, "-validate", "-format", "json", "../../examples/runtime-showcase")
	if asJSON.stdout != asFormat.stdout || asJSON.status != asFormat.status {
		t.Fatalf("-format json differs from -json:\n%s\n---\n%s", asFormat.output(), asJSON.output())
	}
}

// TestValidateJUnitReport: -format junit writes one suite per file, a failing
// case per finding typed by its severity and a passing case for a clean file.
func TestValidateJUnitReport(t *testing.T) {
	binary := buildCLI(t)
	dir := reportWorkspace(t)
	got := checkPathsIn(t, dir, binary, "-validate", "-format", "junit", "models")
	if got.status != exitUnevaluable {
		t.Fatalf("exit %d, want %d\n%s", got.status, exitUnevaluable, got.output())
	}
	var suites junit.Testsuites
	if err := xml.Unmarshal([]byte(got.stdout), &suites); err != nil {
		t.Fatalf("not JUnit XML: %v\n%s", err, got.stdout)
	}
	if len(suites.Suites) != 4 || suites.Tests != 4 || suites.Failures != 2 || suites.Errors != 1 {
		t.Fatalf("totals = %d suites, %d tests, %d failures, %d errors\n%s", len(suites.Suites), suites.Tests, suites.Failures, suites.Errors, got.stdout)
	}
	byName := map[string]junit.Testsuite{}
	for _, s := range suites.Suites {
		byName[s.Name] = s
	}
	clean := byName["models/clean.sysml"]
	if clean.Tests != 1 || clean.Failures != 0 || len(clean.Cases) != 1 || clean.Cases[0].Failure != nil || clean.Cases[0].Skipped != nil {
		t.Fatalf("clean file = %+v", clean)
	}
	run := byName["sysml"]
	if run.Errors != 1 || len(run.Cases) != 1 || run.Cases[0].Error == nil || !strings.Contains(run.Cases[0].Error.Text, "did not analyse cleanly") {
		t.Fatalf("run suite = %+v", run)
	}
	broken := byName["models/broken.sysml"]
	if broken.Failures != len(broken.Cases) || broken.Failures == 0 {
		t.Fatalf("broken file = %+v", broken)
	}
	for _, c := range broken.Cases {
		if c.Failure == nil || c.Failure.Type != "error" || !strings.Contains(c.Failure.Body, "models/broken.sysml:") {
			t.Fatalf("broken case = %+v", c)
		}
	}
	notation := byName["models/notation.sysml"]
	if notation.Failures != 1 || notation.Cases[0].Failure.Type != "warning" || !strings.HasPrefix(notation.Cases[0].Name, "nonstandard-notation at 4:5") {
		t.Fatalf("notation file = %+v", notation)
	}
}

// TestValidateBaselineRoundTrip: the JSON report -o wrote is read back as the
// known findings; they are suppressed wherever their lines moved, and only a
// finding the baseline does not know ends the run.
func TestValidateBaselineRoundTrip(t *testing.T) {
	binary := buildCLI(t)
	dir := reportWorkspace(t)
	strict := []string{"-validate", "-warnings-as-errors"}
	recorded := checkPathsIn(t, dir, binary, append(strict, "-json", "-o", "baseline.json", "models/notation.sysml")...)
	if recorded.status != exitUnevaluable {
		t.Fatalf("recording exit %d\n%s", recorded.status, recorded.output())
	}
	var baseline checkReport
	if data, err := os.ReadFile(filepath.Join(dir, "baseline.json")); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatal(err)
	}
	if len(baseline.Diagnostics) != 1 || baseline.Diagnostics[0].Code != "nonstandard-notation" {
		t.Fatalf("baseline recorded %+v", baseline.Diagnostics)
	}

	again := checkPathsIn(t, dir, binary, append(strict, "-baseline", "baseline.json", "models/notation.sysml")...)
	wantReport(t, again, exitHolds, "1 known finding(s) suppressed by baseline.json", "no errors")
	rejectSeverity(t, again, "warning")

	// The finding moves down two lines and stays known.
	notation := filepath.Join(dir, "models", "notation.sysml")
	if err := os.WriteFile(notation, []byte("\n\n"+reportNotationModel), 0o644); err != nil {
		t.Fatal(err)
	}
	moved := checkPathsIn(t, dir, binary, append(strict, "-baseline", "baseline.json", "-json", "models/notation.sysml")...)
	var report checkReport
	if err := json.Unmarshal([]byte(moved.stdout), &report); err != nil {
		t.Fatalf("%v\n%s", err, moved.output())
	}
	if moved.status != exitHolds || report.Suppressed != 1 || len(report.Diagnostics) != 0 {
		t.Fatalf("after moving lines: exit %d, %d suppressed, %d reported\n%s", moved.status, report.Suppressed, len(report.Diagnostics), moved.output())
	}

	// A finding the baseline does not know is the only one reported, and ends the run.
	fresh := strings.Replace(reportNotationModel, "    state a;\n", "    state a;\n    choice other;\n    transition other then a;\n", 1)
	if err := os.WriteFile(notation, []byte("\n\n"+fresh), 0o644); err != nil {
		t.Fatal(err)
	}
	got := checkPathsIn(t, dir, binary, append(strict, "-baseline", "baseline.json", "models/notation.sysml")...)
	wantReport(t, got, exitUnevaluable, "`choice other;` is an OpenSysML extension", "1 known finding(s) suppressed", "-warnings-as-errors judges")
	rejectReport(t, got, "`choice pick;`")
	asSARIF := checkPathsIn(t, dir, binary, append(strict, "-baseline", "baseline.json", "-format", "sarif", "models/notation.sysml")...)
	log := decodeSARIF(t, []byte(asSARIF.stdout))
	if len(log.Runs[0].Results) != 1 || log.Runs[0].Properties == nil || log.Runs[0].Properties.Suppressed != 1 {
		t.Fatalf("SARIF after baseline: %d results, properties %+v", len(log.Runs[0].Results), log.Runs[0].Properties)
	}
}

// TestValidateBaselineKnowsABlockingError: an error the baseline knows is kept
// out of the report, but analysis still stopped at it, so the run is undecided
// rather than clean, and JUnit says so instead of passing the other files.
func TestValidateBaselineKnowsABlockingError(t *testing.T) {
	binary := buildCLI(t)
	dir := reportWorkspace(t)
	recorded := checkPathsIn(t, dir, binary, "-validate", "-json", "-o", "baseline.json", "models")
	if recorded.status != exitUnevaluable {
		t.Fatalf("recording exit %d\n%s", recorded.status, recorded.output())
	}
	got := checkPathsIn(t, dir, binary, "-validate", "-baseline", "baseline.json", "models")
	wantReport(t, got, exitUnevaluable, "2 known finding(s) suppressed by baseline.json", "an error the baseline knows stopped analysis before the later tiers ran; no check was made")
	rejectReport(t, got, "unresolved reference", "no errors")

	asJUnit := checkPathsIn(t, dir, binary, "-validate", "-baseline", "baseline.json", "-format", "junit", "models")
	var suites junit.Testsuites
	if err := xml.Unmarshal([]byte(asJUnit.stdout), &suites); err != nil {
		t.Fatalf("not JUnit XML: %v\n%s", err, asJUnit.stdout)
	}
	if suites.Errors != 1 || suites.Failures != 0 || suites.Skipped != 3 {
		t.Fatalf("totals = %d errors, %d failures, %d skipped\n%s", suites.Errors, suites.Failures, suites.Skipped, asJUnit.stdout)
	}
	for _, suite := range suites.Suites {
		if suite.Name == "sysml" {
			continue
		}
		if len(suite.Cases) != 1 || suite.Cases[0].Skipped == nil || !strings.Contains(suite.Cases[0].Skipped.Text, "validation did not complete") {
			t.Fatalf("suite %s = %+v", suite.Name, suite.Cases)
		}
	}
}

// TestValidateSARIFCountsUTF16Columns: a column before which the line holds a
// character of two UTF-8 bytes is one less in SARIF, which counts UTF-16 code
// units, than in the JSON report, which counts bytes.
func TestValidateSARIFCountsUTF16Columns(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "u.sysml"), []byte("package U {\n  part def A;\n  /*é*/ part b : Missing;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	asJSON := checkPathsIn(t, dir, binary, "-validate", "-json", "u.sysml")
	var report checkReport
	if err := json.Unmarshal([]byte(asJSON.stdout), &report); err != nil {
		t.Fatalf("%v\n%s", err, asJSON.output())
	}
	asSARIF := checkPathsIn(t, dir, binary, "-validate", "-format", "sarif", "u.sysml")
	log := decodeSARIF(t, []byte(asSARIF.stdout))
	if len(report.Diagnostics) != 1 || len(log.Runs[0].Results) != 1 {
		t.Fatalf("%d JSON findings, %d SARIF results", len(report.Diagnostics), len(log.Runs[0].Results))
	}
	region := log.Runs[0].Results[0].Locations[0].PhysicalLocation.Region
	if region.StartLine != 3 || region.StartColumn != report.Diagnostics[0].Column-1 {
		t.Fatalf("SARIF region %+v for JSON column %d", region, report.Diagnostics[0].Column)
	}
}

// TestValidateWarningsAsErrorsJudgesACheckWarning: a warning a check reports,
// not the model, ends the run under -warnings-as-errors too.
func TestValidateWarningsAsErrorsJudgesACheckWarning(t *testing.T) {
	binary := buildCLI(t)
	model := `package M {
    private import ScalarValues::Real;
    part def Leaf { attribute v : Real = 1.0; }
    part def L3 { part leaves : Leaf[100]; }
    part def L2 { part inner : L3[100]; }
    part def L1 { part inner : L2[100]; }
    part def Craft { part inner : L1[100]; }
    part craft : Craft;
}
`
	lenient := check(t, binary, model, "-instantiate", "M::craft", "-validate")
	wantReport(t, lenient, exitHolds, "materialization is bounded")
	strict := check(t, binary, model, "-instantiate", "M::craft", "-validate", "-warnings-as-errors")
	wantReport(t, strict, exitUnevaluable, "materialization is bounded", "-warnings-as-errors judges an error")
}

// TestValidateWarningsAsErrors: a warning leaves the exit status alone until
// -warnings-as-errors, which ends the run on it while the report keeps the
// warning's level.
func TestValidateWarningsAsErrors(t *testing.T) {
	binary := buildCLI(t)
	dir := reportWorkspace(t)
	lenient := checkPathsIn(t, dir, binary, "-validate", "models/notation.sysml")
	wantReport(t, lenient, exitHolds, "no errors")
	wantSeverity(t, lenient, "warning")
	strict := checkPathsIn(t, dir, binary, "-validate", "-warnings-as-errors", "models/notation.sysml")
	wantReport(t, strict, exitUnevaluable, "-warnings-as-errors judges errors")
	wantSeverity(t, strict, "warning")
	rejectReport(t, strict, "no errors")
	asSARIF := checkPathsIn(t, dir, binary, "-validate", "-warnings-as-errors", "-format", "sarif", "models/notation.sysml")
	log := decodeSARIF(t, []byte(asSARIF.stdout))
	if asSARIF.status != exitUnevaluable || len(log.Runs[0].Results) != 1 || log.Runs[0].Results[0].Level != "warning" {
		t.Fatalf("exit %d, results %+v", asSARIF.status, log.Runs[0].Results)
	}
}

// TestValidateFormatMisuse: a format that is not one, -json beside another
// -format, -o without a document and a report flag without a check are refused.
func TestValidateFormatMisuse(t *testing.T) {
	binary := buildCLI(t)
	dir := reportWorkspace(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-validate", "-format", "yaml", "models"}, "-format yaml is not a report format"},
		{[]string{"-validate", "-json", "-format", "sarif", "models"}, "-json reports JSON; it cannot be combined with -format sarif"},
		{[]string{"-validate", "-o", "report.txt", "models"}, "-o names the file the report document is written to"},
		{[]string{"-format", "sarif", "models"}, "-format reports a check; name one"},
		{[]string{"-format", "text", "models"}, "-format reports a check; name one"},
		{[]string{"-validate", "-json", "-o", "models/clean.sysml", "models"}, "-o models/clean.sysml is a file this run reads"},
		{[]string{"-baseline", "baseline.json", "models"}, "-baseline accepts the known findings of a check"},
		{[]string{"-warnings-as-errors", "models"}, "-warnings-as-errors judges the warnings of a check"},
		{[]string{"-validate", "-baseline", "absent.json", "models"}, "-baseline open absent.json"},
	} {
		got := checkPathsIn(t, dir, binary, tc.args...)
		wantReport(t, got, exitUnevaluable, tc.want)
	}
}

// TestSeverityLineMatchesWithOrWithoutCode: the severity matcher reads a line
// with or without a code after the severity, not one that mentions the word.
func TestSeverityLineMatchesWithOrWithoutCode(t *testing.T) {
	for _, line := range []string{
		"models/notation.sysml:4:5: warning: `choice pick;` is an OpenSysML extension",
		"models/notation.sysml:4:5: warning[nonstandard-notation]: `choice pick;` is an OpenSysML extension",
	} {
		if !severityLine("warning").MatchString(line) {
			t.Errorf("%q not read as a warning line", line)
		}
	}
	for _, line := range []string{
		"sysml: models has warnings, which -warnings-as-errors judges errors; no check was made",
		"models/m.sysml:2:3: error: import without a visibility indicator",
	} {
		if severityLine("warning").MatchString(line) {
			t.Errorf("%q read as a warning line", line)
		}
	}
}
