package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
)

// TestProbesReportTheirConstraint is the evidence behind every ✅/⚠️ row of the
// census: each probe is a minimal violating model, and we must report the
// diagnostic its header names at the severity it names, under a code the
// manifest's row records.
func TestProbesReportTheirConstraint(t *testing.T) {
	root, m := loadCommittedManifest(t)
	probes, err := loadProbes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(probes) == 0 {
		t.Fatalf("no probes under %s", probesDir)
	}
	for _, p := range probes {
		t.Run(filepath.Base(p.Path), func(t *testing.T) {
			row, ok := m.Constraint(p.Constraint)
			if !ok {
				t.Fatalf("%s names %s, which %s does not list", p.Path, p.Constraint, manifest.Path)
			}
			d, seen := probeDiagnostic(t, root, p)
			if d == nil {
				t.Fatalf("%s: expected a %s containing %q for %s; got:\n  %s",
					p.Path, p.Severity, p.Message, p.Constraint, strings.Join(seen, "\n  "))
			}
			if !slicesContains(row.Codes, d.Code) {
				t.Errorf("%s: the diagnostic carries code %q, the row records %q", p.Path, d.Code, row.Codes)
			}
			if !slicesContains(row.Passes, d.Source) {
				t.Errorf("%s: the diagnostic comes from pass %q, the row records %q", p.Path, d.Source, row.Passes)
			}
		})
	}
}

// probeDiagnostic runs a probe and returns the diagnostic its header names, or
// nil with every diagnostic seen instead.
func probeDiagnostic(t *testing.T, root string, p probe) (*diag.Diagnostic, []string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, p.Path))
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(p.Path)
	ws := model.NewWorkspace()
	ws.Open(name, content, 1)
	var seen []string
	for _, d := range ws.Diagnostics(name) {
		if d.Severity.String() == p.Severity && strings.Contains(d.Message, p.Message) {
			return &d, nil
		}
		seen = append(seen, d.Severity.String()+": "+d.Message)
	}
	return nil, seen
}

// probeCodes runs every probe and collects the codes and passes observed per constraint.
func probeCodes(t *testing.T, root string) (codes, passes map[string]map[string]bool) {
	t.Helper()
	probes, err := loadProbes(root)
	if err != nil {
		t.Fatal(err)
	}
	codes, passes = map[string]map[string]bool{}, map[string]map[string]bool{}
	for _, p := range probes {
		d, _ := probeDiagnostic(t, root, p)
		if d == nil {
			continue
		}
		if codes[p.Constraint] == nil {
			codes[p.Constraint], passes[p.Constraint] = map[string]bool{}, map[string]bool{}
		}
		codes[p.Constraint][d.Code] = true
		passes[p.Constraint][d.Source] = true
	}
	return codes, passes
}
