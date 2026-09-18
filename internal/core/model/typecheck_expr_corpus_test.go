package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// exprTypeDiagnostics returns the expression type-checker findings for one
// document, ignoring the relationship-kind tier that shares the "type" source.
func exprTypeDiagnostics(ws *Workspace, name string) []string {
	var out []string
	for _, d := range ws.Diagnostics(name) {
		if d.Code == "type.expr" {
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
		if d.Code == "type.expr" {
			out = append(out, fmt.Sprintf("%s:%d: %s", name, lines.PosAt(d.Span.Offset).Line, d.Message))
		}
	}
	return out
}

// publishedStdlibDefects are the findings the expression type checker reports
// in the standard library as OMG published it: each is a unit the SI or US
// customary library types by a measurement unit of another dimension. The
// errata registry (tools/oracle/errata) corrects the ones with an unambiguous
// reading, so the bundled library the checker loads no longer shows them;
// documentedStdlibDefects have no such reading and stay. Both sets are
// documented in docs/project/omg-issues.md ("Defects in the vendored quantity
// libraries"). The published text itself is never edited.
var (
	correctedStdlibDefects = []string{
		"Domain Libraries/Quantities and Units/SI.sysml:137: cannot bind a measurement reference of dimension T^-2 to a feature typed by TotalMassStoppingPowerUnit",
		"Domain Libraries/Quantities and Units/SI.sysml:247: cannot bind a measurement reference of dimension I^-2·L^6·T^-2 to a feature typed by HallCoefficientUnit",
		"Domain Libraries/Quantities and Units/USCustomaryUnits.sysml:255: cannot bind a value of dimension Θ^-1 to a feature typed by ThermodynamicTemperatureValue (dimension Θ)",
	}
	documentedStdlibDefects = []string{
		"Domain Libraries/Quantities and Units/SI.sysml:149: cannot bind a measurement reference of dimension L^4·M^2·T^-2 to a feature typed by TotalAngularMomentumUnit",
		"Domain Libraries/Quantities and Units/SI.sysml:163: cannot bind a measurement reference of dimension L^-10·M^-2·T^4 to a feature typed by EnergyDensityOfStatesUnit",
		"Domain Libraries/Quantities and Units/SI.sysml:233: cannot bind a measurement reference of dimension I·L^2 to a feature typed by MagneticDipoleMomentUnit",
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
	checkStdlibExprTypeFindings(t, libs.DefaultSource(), documentedStdlibDefects)
}

// TestExprTypeCheckPublishedStdlibDefects pins the checker's verdict on the
// text as published: every corrected defect is a defect the checker finds
// there, so a correction is only ever declared for a line the checker rejects.
func TestExprTypeCheckPublishedStdlibDefects(t *testing.T) {
	checkStdlibExprTypeFindings(t, libs.EmbeddedSource(), publishedStdlibDefects)
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
		filepath.Join("..", "runtime", "testdata"),
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
				if info.Name() == "pilot-corpora" {
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
