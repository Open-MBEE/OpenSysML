package model

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// extensionModel uses notation the pinned grammars do not admit, in the two
// codes strict mode escalates.
const extensionModel = "package P { attribute def Alarm; state def S { state a { defer Alarm; } } }"

func TestWorkspaceDefaultsToTheDefaultMode(t *testing.T) {
	ws := NewWorkspace()
	if ws.ConformanceMode() != diag.ConformanceDefault {
		t.Fatalf("mode = %v, want default", ws.ConformanceMode())
	}
	ws.Open("a.sysml", []byte(extensionModel), 1)
	for _, d := range ws.Diagnostics("a.sysml") {
		if d.Severity == diag.SeverityError {
			t.Fatalf("default mode must accept our notation, got %+v", d)
		}
	}
}

func TestWorkspaceStrictModeRejectsExtensionNotation(t *testing.T) {
	ws := NewWorkspace(WithConformanceMode(diag.ConformanceStrict))
	ws.Open("a.sysml", []byte(extensionModel), 1)
	var errs int
	for _, d := range ws.Diagnostics("a.sysml") {
		if d.Severity == diag.SeverityError && d.Code == passes.CodeNonstandardNotation {
			errs++
		}
	}
	if errs == 0 {
		t.Fatalf("strict mode reported no nonstandard-notation error: %+v", ws.Diagnostics("a.sysml"))
	}
}

// Changing the mode after diagnostics were served must re-analyse: a cached
// warning would answer the strict question with the default one's verdict.
func TestSetConformanceModeReanalyses(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte(extensionModel), 1)
	if severities := conformanceSeverities(ws, "a.sysml"); severities[diag.SeverityError] != 0 {
		t.Fatalf("default mode errored: %+v", ws.Diagnostics("a.sysml"))
	}
	ws.SetConformanceMode(diag.ConformanceStrict)
	if conformanceSeverities(ws, "a.sysml")[diag.SeverityError] == 0 {
		t.Fatalf("after switching to strict: %+v", ws.Diagnostics("a.sysml"))
	}
	ws.SetConformanceMode(diag.ConformanceDefault)
	if conformanceSeverities(ws, "a.sysml")[diag.SeverityError] != 0 {
		t.Fatalf("after switching back to default: %+v", ws.Diagnostics("a.sysml"))
	}
}

func conformanceSeverities(ws *Workspace, name string) map[diag.Severity]int {
	out := map[diag.Severity]int{}
	for _, d := range ws.Diagnostics(name) {
		out[d.Severity]++
	}
	return out
}

// The strongest available guarantee that the option is additive: over every
// example the repository ships, a workspace told to use the default mode and one
// told nothing report identical diagnostics.
func TestDefaultModeIsUnchangedOverTheExamples(t *testing.T) {
	files := exampleFiles(t)
	if len(files) == 0 {
		t.Fatalf("no examples found under %s", examplesDir)
	}
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(examplesDir, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		implicit := NewWorkspace()
		implicit.Open(rel, content, 1)
		named := NewWorkspace(WithConformanceMode(diag.ConformanceDefault))
		named.Open(rel, content, 1)
		want, got := implicit.Diagnostics(rel), named.Diagnostics(rel)
		if len(want) != len(got) {
			t.Fatalf("%s: %d diagnostic(s) with the option, %d without", rel, len(got), len(want))
		}
		for i := range want {
			if fmt.Sprintf("%+v", want[i]) != fmt.Sprintf("%+v", got[i]) {
				t.Errorf("%s: diagnostic %d differs: %+v vs %+v", rel, i, want[i], got[i])
			}
		}
	}
}
