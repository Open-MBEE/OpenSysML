package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Taking a transition and restoring the executor allocates nothing: a state
// machine that loops on a transition did so twice per step (two closures).
func TestTakingATransitionAllocatesNothing(t *testing.T) {
	src := `package T {
		private import ScalarValues::*;
		state def SM {
			attribute count : Integer = 0;
			entry; then s0;
			state s0;
			accept after 1 [SI::s] if count < 3 then s1;
			accept after 1 [SI::s] if count >= 3 then done;
			state s1 {
				entry assign count := count + 1;
			}
			accept after 1 [SI::s] then s0;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "SM", ast.DefState)
	if sym == nil {
		t.Fatal("state def SM not found")
	}
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	var trans []*lower.Transition
	for _, out := range exec.Graph().Transitions {
		trans = append(trans, out...)
	}
	if len(trans) == 0 {
		t.Fatal("no transitions lowered")
	}
	got := testing.AllocsPerRun(100, func() {
		for _, tr := range trans {
			exec.taking(tr, nil).restore()
		}
	})
	if got != 0 {
		t.Errorf("taking %d transitions allocates %.2f objects, want 0", len(trans), got)
	}
}
