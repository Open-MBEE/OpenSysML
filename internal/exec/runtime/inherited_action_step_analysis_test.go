package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func TestInheritedActionStepFixturesAnalyseCleanly(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "conformance", "inherited_action_steps_*.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no inherited action step fixtures found")
	}
	paths = append(paths, filepath.Join("testdata", "conformance", "action_invoked_node_body_writes_output.sysml"))

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			sf := source.New(path, data)
			p := parser.New(sf)
			root := p.ParseFile()

			idx := libs.NewModelIndex()
			idx.AddDocument(sf.Name(), root)
			idx.ExpandWildcardImports()

			diags := passes.Analyze(sf.Name(), root, parser.AsDiagnostics(p.Diagnostics, p.Warnings), idx)
			for _, d := range diags {
				if d.Severity == diag.SeverityError {
					t.Errorf("%s: %s", d.Code, d.Message)
				}
			}
		})
	}
}
