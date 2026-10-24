package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// TestRootFrameHeldIndexIsSizedUpFront pins that declaring the root frame's
// features into an empty index costs one allocation more than declaring them
// into an index already sized for them: the map itself, never its regrowth.
func TestRootFrameHeldIndexIsSizedUpFront(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", parser.New(source.New("<test>", []byte(`package test {
		private import ScalarValues::*;
		action def Proc {
			in a : Integer; in b : Integer; out c : Integer;
			attribute d : Integer = 1; attribute e : Integer = 2;
			assign c := a + b + d + e;
		}
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 1_000_000)
	exec, err := newActionExecutor(ctx, idx.LookupQualified("test::Proc")[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	root := exec.newRootFrame(exec.performanceShape())
	root.ownShape()
	held := len(root.declared)
	if held < 5 {
		t.Fatalf("root frame holds %d features, want the action's five and its inherited ones", held)
	}

	presized := testing.AllocsPerRun(50, func() {
		root.declared = make(map[*symbols.Symbol]string, held)
		exec.declareRootFeatures(root, exec.features)
	})
	empty := testing.AllocsPerRun(50, func() {
		root.declared = nil
		exec.declareRootFeatures(root, exec.features)
	})
	if len(root.declared) != held {
		t.Fatalf("re-declaring holds %d features, want %d", len(root.declared), held)
	}
	if empty > presized+1 {
		t.Errorf("declaring %d features into an empty index allocates %.0f objects against %.0f into a sized one: the index grows entry by entry", held, empty, presized)
	}
}
