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

// validFixtureSets lists the fixtures that are valid models, grouped as the
// view tests load them, so their renderings are gated on models that pass
// validation. A sibling named in loaded but not in files is loaded for its
// declarations only: its own errors are deliberate and documented at its top.
// A file in pinned carries exactly one error of the code named, deliberate and
// documented at its top, and is held to zero others. A fixture absent here
// renders an error on purpose, and says so at its top.
var validFixtureSets = []struct {
	files  []string
	loaded []string
	pinned map[string]string
}{
	{files: []string{
		"case.sysml", "mixed.sysml", "mixed-inherited-actor.sysml", "mixed-inherited-reference.sysml",
		"mixed-review.sysml", "view-role-inheritance.sysml",
	}},
	{files: []string{"anonymous-actions.sysml"}},
	{files: []string{"boundary-parameters.sysml"}},
	{files: []string{"cameo-behavior.sysml"}},
	{files: []string{"depth-tree.sysml"}},
	{files: []string{"interconnection.sysml"}},
	{files: []string{"layout-usages.sysml"}, loaded: []string{"layout.sysml"}},
	{files: []string{"positioned-views.sysml"}},
	{files: []string{"regions-usages.sysml", "regions.sysml"}, pinned: map[string]string{"regions.sysml": "no-outgoing-transition"}},
	{files: []string{"tree-edges.sysml"}},
	{files: []string{"typings.sysml"}},
}

// TestFixturesHaveNoErrorDiagnostics holds every fixture of validFixtureSets to
// zero error diagnostics from the validation passes over the standard library
// (the one pinned error excepted), the set's siblings loaded alongside as the
// view tests load them.
func TestFixturesHaveNoErrorDiagnostics(t *testing.T) {
	for _, set := range validFixtureSets {
		files := append(append([]string{}, set.files...), set.loaded...)
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

		for _, file := range set.files {
			pinnedSeen := 0
			for _, diagnostic := range passes.Analyze(file, roots[file], parseDiagnostics[file], idx) {
				if diagnostic.Severity != diag.SeverityError {
					continue
				}
				if code, ok := set.pinned[file]; ok && diagnostic.Code == code && pinnedSeen == 0 {
					pinnedSeen++
					continue
				}
				t.Errorf("%s: %s", file, diagnostic.Message)
			}
			if code, ok := set.pinned[file]; ok && pinnedSeen == 0 {
				t.Errorf("%s: no %s error is left; its deliberate error is gone, so gate it fully", file, code)
			}
		}
	}
}
