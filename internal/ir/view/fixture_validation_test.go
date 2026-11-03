package view_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// invalidFixtures render an error on purpose and say so at their top; every other fixture under
// testdata is a valid model and is gated, so a new fixture is gated unless it is listed here.
var invalidFixtures = map[string]bool{
	"empty-reasons.sysml":      true,
	"layout.sysml":             true,
	"pictures.sysml":           true,
	"state-refused.sysml":      true,
	"typings-unresolved.sysml": true,
}

// pinnedErrors names the one deliberate error a gated fixture carries, documented at its top; the
// fixture is held to exactly one error of that code and zero others.
var pinnedErrors = map[string]string{
	"regions.sysml": "no-outgoing-transition",
}

// siblingSets lists the fixtures the view tests load together; a fixture in no set is loaded alone.
var siblingSets = [][]string{
	{"case.sysml", "mixed.sysml", "mixed-inherited-actor.sysml", "mixed-inherited-reference.sysml", "mixed-review.sysml", "view-role-inheritance.sysml"},
	{"layout-usages.sysml", "layout.sysml"},
	{"regions-usages.sysml", "regions.sysml"},
}

// TestFixturesHaveNoErrorDiagnostics sweeps every fixture under testdata and holds each one not in
// invalidFixtures to zero error diagnostics from the validation passes, its pinned error excepted.
func TestFixturesHaveNoErrorDiagnostics(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "*.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	exists := make(map[string]bool, len(paths))
	for _, path := range paths {
		exists[filepath.Base(path)] = true
	}
	for name := range invalidFixtures {
		if !exists[name] {
			t.Errorf("invalidFixtures names %s, which is not under testdata", name)
		}
	}
	for name := range pinnedErrors {
		if !exists[name] {
			t.Errorf("pinnedErrors names %s, which is not under testdata", name)
		}
	}
	grouped := make(map[string]bool)
	sets := [][]string{}
	for _, set := range siblingSets {
		for _, name := range set {
			if !exists[name] {
				t.Errorf("siblingSets names %s, which is not under testdata", name)
			}
			grouped[name] = true
		}
		sets = append(sets, set)
	}
	for _, path := range paths {
		if name := filepath.Base(path); !grouped[name] {
			sets = append(sets, []string{name})
		}
	}

	for _, files := range sets {
		idx := libs.NewModelIndex()
		roots := make(map[string]*ast.RootNamespace, len(files))
		parseDiagnostics := make(map[string][]diag.Diagnostic, len(files))
		for _, file := range files {
			path := filepath.Join("testdata", file)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			p := parser.New(source.New(file, content))
			root := p.ParseFile()
			roots[file] = root
			parseDiagnostics[file] = parser.AsDiagnostics(p.Diagnostics, p.Warnings)
			idx.AddDocument(file, root)
		}
		idx.ExpandWildcardImports()

		for _, file := range files {
			if invalidFixtures[file] {
				continue
			}
			pinnedSeen := 0
			for _, diagnostic := range passes.Analyze(file, roots[file], parseDiagnostics[file], idx) {
				if diagnostic.Severity != diag.SeverityError {
					continue
				}
				if code, ok := pinnedErrors[file]; ok && diagnostic.Code == code && pinnedSeen == 0 {
					pinnedSeen++
					continue
				}
				t.Errorf("%s: %s", file, diagnostic.Message)
			}
			if code, ok := pinnedErrors[file]; ok && pinnedSeen == 0 {
				t.Errorf("%s: no %s error is left; its deliberate error is gone, so gate it fully", file, code)
			}
		}
	}
}
