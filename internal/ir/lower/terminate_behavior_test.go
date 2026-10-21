package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A terminate node naming an occurrence lowers, for a state transition ending at
// it, to an ordered behavior — its body as one block, then the terminate effect —
// and to the completion vertex of the body declaring it, the one `done` there is.
func TestNamedTerminateNodeLowersToOrderedBehaviorAndCompletion(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	item def Abort;
	part def Worker;
	part def Controller {
		part worker : Worker;
		attribute alarm : Integer = 0;
		exhibit state control {
			entry; then busy;
			state busy;
			transition first busy accept Abort then halt;
			action halt terminate worker { assign alarm := 1; }
			transition first busy then done;
		}
	}
}`
	p := parser.New(source.New("p.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("p.sysml", root)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot("p.sysml").LookupLocal("P")
	if !ok {
		t.Fatal("package P not indexed")
	}
	controller, ok := pkg.Scope.LookupLocal("Controller")
	if !ok {
		t.Fatal("part def Controller not indexed")
	}
	machine, ok := controller.Scope.LookupLocal("control")
	if !ok {
		t.Fatal("state control not indexed")
	}
	graph, err := ToStateGraphWithEndpoints(machine.Decl, machine.Scope, NewLibraryStateTypes(resolve.New(idx)))
	if err != nil {
		t.Fatalf("ToStateGraphWithEndpoints failed: %v", err)
	}
	if len(graph.Terminates) != 1 {
		t.Fatalf("got %d terminate usages, want 1", len(graph.Terminates))
	}
	halt := graph.Terminates[0]
	behavior, ok := graph.TerminateBehaviors[halt]
	if !ok {
		t.Fatal("no behavior recorded for the named terminate node")
	}
	if !behavior.Ordered {
		t.Error("the terminate node's behavior must run its body before its terminate: Ordered = false")
	}
	if len(behavior.Body) != 2 {
		t.Fatalf("behavior body has %d statements, want the body block then the terminate", len(behavior.Body))
	}
	if _, isBlock := behavior.Body[0].(Block); !isBlock {
		t.Errorf("behavior.Body[0] = %T, want the node's body as a Block", behavior.Body[0])
	}
	if effect, isEffect := behavior.Body[1].(Effect); !isEffect || effect.Kind != EffectTerminate || effect.Terminates != TerminateOccurrence {
		t.Errorf("behavior.Body[1] = %#v, want a terminate effect naming the occurrence", behavior.Body[1])
	}
	completion := graph.TerminateCompletion[halt]
	if completion == nil || !graph.Completes(completion) {
		t.Fatalf("completion vertex = %v, want the body's completion vertex", completion)
	}
	for _, trans := range graph.Transitions[stateNamed(graph, "busy")] {
		if trans.Trigger == nil {
			if done, isState := trans.Target.(*ast.StateNode); !isState || done != completion {
				t.Errorf("`then done` from busy targets %v, want the same completion vertex the terminate node hands on to", trans.Target)
			}
		}
	}
}
