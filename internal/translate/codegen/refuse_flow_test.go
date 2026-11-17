package codegen_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const flowModel = `package test {
    private import ScalarValues::*;
    item def Ping { attribute v : Integer; }
    action def Bump { in a : Integer; out o : Integer = a + 1; }
    part def Sink { attribute seen : Integer := 0; }
    part sink : Sink;
    calc def Nested { in n : Integer; attribute x : Integer := n; action a { assign x := 7; } return : Integer = x; }
    calc def PerformsRead { in n : Integer; perform action b : Bump { in a = n; } return : Integer = b.o; }
    calc def BranchTerminates { in n : Integer; attribute x : Integer := n; if n > 0 { terminate; } return : Integer = x; }
    part def Host { attribute total : Integer := 0; calc def Own { in n : Integer; assign total := n; return : Integer = n; } }
    calc def NoneOf { in n : Integer; attribute x : Integer := n; action a [0] { assign x := 7; } return : Integer = x; }
    calc def TwoOf { in n : Integer; attribute x : Integer := n; action a [2] { assign x := 7; } return : Integer = x; }
    calc def Sends { in n : Integer; send new Ping(n) to sink; return : Integer = n; }
    calc def Accepts { in n : Integer; accept p : Ping; return : Integer = n; }
    calc def LoopAccepts { in n : Integer; attribute x : Integer := n; while x < 3 { accept p : Ping; assign x := x + 1; } return : Integer = x; }
    calc def Terminates { in n : Integer; attribute x : Integer := n; terminate; return : Integer = x; }
    calc def NestedTerminates { in n : Integer; attribute x : Integer := n; action a { terminate; } return : Integer = x; }
    calc def Performs { in n : Integer; attribute x : Integer := n; perform action b : Bump { in a = n; } return : Integer = x; }
    calc def Reordered { in n : Integer; attribute x : Integer := n; action b { assign x := x * 2; } action a { assign x := x + 5; } first a then b; return : Integer = x; }
    calc def WritesOutside { in n : Integer; assign sink.seen := n; return : Integer = n; }
}
`

// TestCompileRefusesBodyEffects covers a calc whose body acts beyond its statements:
// each effect is refused with a typed error, never dropped from the program.
func TestCompileRefusesBodyEffects(t *testing.T) {
	const path = "flow.sysml"
	idx := libs.NewModelIndex()
	idx.AddDocument(path, parser.New(source.New(path, []byte(flowModel))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	root, ok := idx.DocumentRoot(path).LookupLocal("test")
	if !ok || root.Scope == nil {
		t.Fatal("package test not found")
	}
	pkg := root.Scope
	compiler := codegen.New(model, resolver)
	for _, target := range []codegen.Target{codegen.TargetGo, codegen.TargetC} {
		t.Run(string(target), func(t *testing.T) {
			// The C target refuses Integer `+`, so the body that compiles declares no arithmetic.
			if _, err := compiler.Compile(recordCalc(t, pkg, "Nested"), target); err != nil {
				t.Errorf("Nested: %v; want it compiled", err)
			}
			for _, c := range []struct{ calc, want string }{
				{"Sends", "a send, which posts a message outside the calc"},
				{"Accepts", "an accept, which waits for a message"},
				{"LoopAccepts", "an accept, which waits for a message"},
				{"Terminates", "a statement acting outside the calc"},
				{"NestedTerminates", "a statement acting outside the calc"},
				{"Performs", "a flow between the pins of action nodes"},
				{"NoneOf", "an action node stating how many times it is performed"},
				{"TwoOf", "an action node stating how many times it is performed"},
				{"Reordered", "a token flow ordering its steps other than as they are declared"},
				{"PerformsRead", "a flow between the pins of action nodes"},
				{"BranchTerminates", "a statement acting outside the calc"},
				{"WritesOutside", "assignment through a feature chain"},
				{"Own", "assignment to total, which the body does not declare"},
			} {
				scope := pkg
				if c.calc == "Own" {
					scope = recordCalc(t, pkg, "Host").Scope
				}
				_, err := compiler.Compile(recordCalc(t, scope, c.calc), target)
				var unsupported *codegen.UnsupportedError
				if !errors.As(err, &unsupported) || !errors.Is(err, codegen.ErrUnsupported) || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v; want a typed refusal naming %q", c.calc, err, c.want)
				}
			}
		})
	}
}
