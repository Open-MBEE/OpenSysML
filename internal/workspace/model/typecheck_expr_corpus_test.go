package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs/errata"
)

// exprTypeCodes are the codes of the expression type checker's findings, which
// share the "type" source with the relationship-kind rules left out here.
var exprTypeCodes = map[string]bool{
	"binding-connector-type-conformance": true, "feature-value-count": true, "unique-feature-values": true,
	"sequence-index": true, "operator-operand-type": true, "comparison-constant": true,
	"invocation-argument": true, "constructor-argument": true,
	"constructor-expression-no-duplicate-feature-redefinition": true, "connector-end-index": true,
	"condition-boolean": true, "incommensurable-binding": true, "incommensurable-operands": true,
}

// exprTypeDiagnostics returns the expression type-checker findings for one
// document, ignoring the relationship-kind tier that shares the "type" source.
func exprTypeDiagnostics(ws *Workspace, name string) []string {
	var out []string
	for _, d := range ws.Diagnostics(name) {
		if exprTypeCodes[d.Code] {
			out = append(out, name+": "+d.Message)
		}
	}
	return out
}

// exprTypeDiagnosticLines is exprTypeDiagnostics with each finding located at
// its 1-based line, for pinning a finding to the declaration it reports.
func exprTypeDiagnosticLines(ws *Workspace, name string, content []byte) []string {
	lines := source.New(name, content).Lines()
	var out []string
	for _, d := range ws.Diagnostics(name) {
		if exprTypeCodes[d.Code] {
			out = append(out, fmt.Sprintf("%s:%d: %s", name, lines.PosAt(d.Span.Offset).Line, d.Message))
		}
	}
	return out
}

// publishedStdlibDefects are the findings the expression type checker reports
// in the standard library as OMG published it: each is a unit the SI or US
// customary library types by a measurement unit of another dimension. The
// errata registry (internal/workspace/libs/errata) corrects the ones with an
// unambiguous reading, so the bundled library the checker loads no longer shows
// them; documentedStdlibDefects have no such reading and stay. Both sets are
// documented in docs/project/omg-issues.md ("Defects in the vendored quantity
// libraries"). The published text itself is never edited.
var (
	correctedStdlibDefects = []string{
		"Domain Libraries/Quantities and Units/SI.sysml:137: cannot bind a measurement reference of dimension T^-2 to a feature typed by TotalMassStoppingPowerUnit",
		"Domain Libraries/Quantities and Units/SI.sysml:233: cannot bind a measurement reference of dimension I·L^2 to a feature typed by MagneticDipoleMomentUnit",
		"Domain Libraries/Quantities and Units/SI.sysml:247: cannot bind a measurement reference of dimension I^-2·L^6·T^-2 to a feature typed by HallCoefficientUnit",
		"Domain Libraries/Quantities and Units/USCustomaryUnits.sysml:255: cannot bind a value of dimension Θ^-1 to a feature typed by ThermodynamicTemperatureValue (dimension Θ)",
	}
	documentedStdlibDefects = []string{
		"Domain Libraries/Quantities and Units/SI.sysml:149: cannot bind a measurement reference of dimension L^4·M^2·T^-2 to a feature typed by TotalAngularMomentumUnit",
		"Domain Libraries/Quantities and Units/SI.sysml:163: cannot bind a measurement reference of dimension L^-10·M^-2·T^4 to a feature typed by EnergyDensityOfStatesUnit",
		"Domain Libraries/Quantities and Units/SI.sysml:239: cannot bind a measurement reference of dimension L^2·T^-3 to a feature typed by DoseEquivalentUnit",
		"Domain Libraries/Quantities and Units/SI.sysml:286: cannot bind a measurement reference of dimension L^2·T^-3 to a feature typed by DoseEquivalentUnit",
		"Domain Libraries/Quantities and Units/SI.sysml:299: cannot bind a measurement reference of dimension L^2·T^-3 to a feature typed by DoseEquivalentUnit",
	}
	publishedStdlibDefects = append(append([]string{}, correctedStdlibDefects...), documentedStdlibDefects...)
)

// TestExprTypeCheckNoStdlibFalsePositives guards the expression type checker
// against over-reporting: every finding in the bundled standard library must
// be one of the documented defects the errata registry leaves uncorrected,
// and each of those must still be found, so the pin cannot rot into silence.
// The corrected defects must not be reported: that is the corrected text
// reaching the checker.
func TestExprTypeCheckNoStdlibFalsePositives(t *testing.T) {
	t.Parallel()
	checkStdlibExprTypeFindings(t, libs.DefaultSource(), documentedStdlibDefects)
}

// TestExprTypeCheckPublishedStdlibDefects pins the checker's verdict on the
// text as published: every corrected defect is a defect the checker finds
// there.
func TestExprTypeCheckPublishedStdlibDefects(t *testing.T) {
	t.Parallel()
	checkStdlibExprTypeFindings(t, libs.EmbeddedSource(), publishedStdlibDefects)
}

// hiddenNameStdlibCorrections are corrections the checker has no finding for: lines
// naming a member two imports bring, which KerML 7.2.5.4 hides, qualified by the one meant.
var hiddenNameStdlibCorrections = []string{
	"Domain Libraries/Quantities and Units/SI.sysml:303",
}

// validationStdlibCorrections are corrections neither the checker nor the
// hidden-name list accounts for: lines whose correction, with the others naming
// the same finding, clears a validation finding the published library reports.
// Each is keyed by the corrected line and names that finding as "<file>:<line>:
// <code>". Empty since the 2026-09 library published the Flows.sysml repair
// (Message and messages own usage ends) that the three former entries carried.
var validationStdlibCorrections = map[string]string{}

// TestStdlibCorrectionsAreAccountedFor pins the registry's library corrections as an exact
// set: each is a line the checker rejects as published, a listed hidden-name correction, or
// a listed line of a repair that clears a validation finding.
func TestStdlibCorrectionsAreAccountedFor(t *testing.T) {
	t.Parallel()
	overlay, err := errata.Library()
	if err != nil {
		t.Fatal(err)
	}
	accounted := map[string]bool{}
	for _, finding := range correctedStdlibDefects {
		accounted[finding[:strings.Index(finding, ": ")]] = false
	}
	for _, line := range hiddenNameStdlibCorrections {
		if _, dup := accounted[line]; dup {
			t.Errorf("%s is listed as both a checker finding and a hidden-name correction", line)
		}
		accounted[line] = false
	}
	for line := range validationStdlibCorrections {
		if _, dup := accounted[line]; dup {
			t.Errorf("%s is listed twice over as a correction", line)
		}
		accounted[line] = false
	}
	for _, entry := range overlay.Corrections() {
		at := fmt.Sprintf("%s:%d", strings.TrimPrefix(entry.Path, errata.LibraryRoot+"/"), entry.Line)
		if _, ok := accounted[at]; !ok {
			t.Errorf("correction %s at %s is neither a checker finding, a hidden-name correction nor a validation repair", entry.ID, at)
			continue
		}
		accounted[at] = true
	}
	for at, seen := range accounted {
		if !seen {
			t.Errorf("%s is listed as corrected, the registry declares no correction there", at)
		}
	}
}

// TestStdlibValidationCorrectionsClearTheirFindings pins the validation findings
// the listed repairs answer: each is reported over the published text of its
// file, and none over the bundled library.
func TestStdlibValidationCorrectionsClearTheirFindings(t *testing.T) {
	t.Parallel()
	findings := map[string]bool{}
	for _, finding := range validationStdlibCorrections {
		findings[finding] = true
	}
	for finding := range findings {
		name := finding[:strings.Index(finding, ":")]
		published := stdlibFindingsAt(t, libs.EmbeddedSource(), name)
		if !published[finding] {
			t.Errorf("the published %s does not report %q; found %v", name, finding, sortedKeys(published))
		}
		if bundled := stdlibFindingsAt(t, libs.DefaultSource(), name); bundled[finding] {
			t.Errorf("the bundled %s still reports %q", name, finding)
		}
	}
}

// stdlibFindingsAt opens the file name of src in the bundled file's place and
// returns the diagnostics reported in it as "<file>:<line>: <code>".
func stdlibFindingsAt(t *testing.T, src libs.Source, name string) map[string]bool {
	t.Helper()
	data, err := src.Read(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	ws := NewWorkspace()
	ws.Open(name, data, 1)
	defer ws.Close(name)
	lines := source.New(name, data).Lines()
	found := map[string]bool{}
	for _, d := range ws.Diagnostics(name) {
		found[fmt.Sprintf("%s:%d: %s", name, lines.PosAt(d.Span.Offset).Line, d.Code)] = true
	}
	return found
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// checkStdlibExprTypeFindings opens each file of src under its own name, which
// puts it in the bundled file's place, and requires the type.expr findings
// over all of them to be exactly want.
func checkStdlibExprTypeFindings(t *testing.T, src libs.Source, want []string) {
	t.Helper()
	ws := NewWorkspace()
	var found []string
	for _, name := range src.List() {
		data, err := src.Read(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		ws.Open(name, data, 1)
		found = append(found, exprTypeDiagnosticLines(ws, name, data)...)
		ws.Close(name)
	}
	documented := map[string]bool{}
	for _, defect := range want {
		documented[defect] = false
	}
	var unexpected []string
	for _, finding := range found {
		if _, ok := documented[finding]; !ok {
			unexpected = append(unexpected, finding)
			continue
		}
		documented[finding] = true
	}
	if len(unexpected) != 0 {
		t.Errorf("expression type checker reported %d unexpected finding(s) in the standard library:\n%s",
			len(unexpected), strings.Join(unexpected, "\n"))
	}
	for _, defect := range want {
		if !documented[defect] {
			t.Errorf("library defect no longer reported: %s", defect)
		}
	}
}

// TestExprTypeCheckStdlibFindingsNeedTheLibrary proves the gate above is not
// vacuous: the quantity libraries the SI units are judged against stay in the
// workspace after a file opened under a bundled name is closed.
func TestExprTypeCheckStdlibFindingsNeedTheLibrary(t *testing.T) {
	const isq = "Domain Libraries/Quantities and Units/ISQBase.sysml"
	src := libs.DefaultSource()
	data, err := src.Read(isq)
	if err != nil {
		t.Fatalf("read %s: %v", isq, err)
	}
	ws := NewWorkspace()
	ws.Open(isq, data, 1)
	ws.Close(isq)
	if syms := ws.LookupQualified("ISQBase::LengthUnit"); len(syms) != 1 {
		t.Fatalf("ISQBase::LengthUnit = %d symbols after closing %s, want the bundled one", len(syms), isq)
	}
}

// TestExprTypeCheckNoExampleFalsePositives runs the same guard over the
// well-formed models the repository ships as examples and runtime fixtures.
func TestExprTypeCheckNoExampleFalsePositives(t *testing.T) {
	roots := []string{
		filepath.Join("..", "..", "..", "examples"),
		filepath.Join("..", "..", "exec", "runtime", "testdata"),
	}
	ws := NewWorkspace()
	var found []string
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// The pilot corpora are third-party models downloaded under
			// examples/, not models this repository ships; the advisory
			// differential harness reports on those.
			if info.IsDir() {
				if info.Name() == "pilot-corpora" || info.Name() == "spec-examples" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".sysml") && !strings.HasSuffix(path, ".kerml") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			ws.Open(path, data, 1)
			found = append(found, exprTypeDiagnostics(ws, path)...)
			ws.Close(path)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(found) != 0 {
		t.Fatalf("expression type checker reported %d finding(s) in example models:\n%s",
			len(found), strings.Join(found, "\n"))
	}
}
