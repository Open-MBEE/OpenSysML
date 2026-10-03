package runtime_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// TestRuntimeRobustnessRecords covers the boundary of a compiled program at a
// record: a record result or command-line parameter is refused with a typed
// error naming it, while a feature read off a record leaves the program.
func TestRuntimeRobustnessRecords(t *testing.T) {
	compiler, pkg := recordCompiler(t)
	for _, target := range []codegen.Target{codegen.TargetGo, codegen.TargetC} {
		t.Run(string(target), func(t *testing.T) {
			for _, c := range []struct{ calc, want string }{
				{"Make", "result: a test::Point, a record"},
				{"MakeMany", "result: a test::Point[0..*], a record"},
				{"Takes", "parameter p takes a test::Point, a record"},
				{"TakesMany", "parameter ps takes a test::Point[0..*], a record"},
			} {
				_, err := compiler.Compile(recordCalc(t, pkg, c.calc), target)
				var unsupported *codegen.UnsupportedError
				if !errors.As(err, &unsupported) || !errors.Is(err, codegen.ErrUnsupported) || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v; want a typed refusal naming %q", c.calc, err, c.want)
				}
			}
			for _, calc := range []string{"ReadOff", "PassesInside"} {
				if _, err := compiler.Compile(recordCalc(t, pkg, calc), target); err != nil {
					t.Errorf("%s: %v; want it compiled", calc, err)
				}
			}
		})
	}
}

const recordModel = `
	package test {
		private import ScalarValues::*;
		attribute def Point { attribute x : Real; attribute y : Real; }
		calc def Make { in a : Real; return : Point = new Point(a, a); }
		calc def MakeMany { in a : Real; return : Point[0..*] = (new Point(a, a), new Point(a, 1.0)); }
		calc def Takes { in p : Point; return : Real = p.x; }
		calc def TakesMany { in ps : Point[0..*]; return : Real = 1.0; }
		calc def ReadOff { in a : Real; return : Real = Make(a).y; }
		calc def PassesInside { in a : Real; return : Real = Takes(new Point(a, 2.0)); }
	}
`

// recordCompiler builds recordModel with the libraries, returning the compiler and package scope.
func recordCompiler(t *testing.T) (*codegen.Compiler, *symbols.Scope) {
	t.Helper()
	const path = "records.sysml"
	idx := libs.NewModelIndex()
	idx.AddDocument(path, parser.New(source.New(path, []byte(recordModel))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	pkg, ok := idx.DocumentRoot(path).LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("package test not found")
	}
	return codegen.New(model, resolver), pkg.Scope
}

// recordCalc finds calc def name in pkg.
func recordCalc(t *testing.T, pkg *symbols.Scope, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := pkg.LookupLocal(name)
	if !ok {
		t.Fatalf("calc %s not found", name)
	}
	return sym
}
