package runtime

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// actionLoopPassAllocs is what one more pass of an action's `for` loop costs
// in allocations: the difference between runs of 2n and n passes, per pass.
func actionLoopPassAllocs(t *testing.T, n int) float64 {
	t.Helper()
	run := func(passes int) float64 {
		src := fmt.Sprintf(`package T {
			private import ScalarValues::*;
			action run {
				attribute total : Integer = 0;
				first start;
				action iterate {
					for i in 1..%d {
						assign total := total + i;
					}
				}
				done;
				succession first start then iterate;
				succession first iterate then done;
			}
		}`, passes)
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "run", ast.DefAction)
		if sym == nil {
			t.Fatal("action run not found")
		}
		return testing.AllocsPerRun(10, func() {
			if _, err := ctx.ExecuteAction(sym); err != nil {
				t.Fatal(err)
			}
		})
	}
	return (run(2*n) - run(n)) / float64(n)
}

// A pass of an action's loop assigns through a footprint stamped for the
// performance once, not re-stamped (two slices cloned) per pass: bound below.
func TestActionLoopPassDoesNotRestampStatementFootprint(t *testing.T) {
	const bound = 4.5
	got := actionLoopPassAllocs(t, 200)
	t.Logf("one more pass allocates %.2f objects", got)
	if got > bound {
		t.Errorf("one more pass of the loop allocates %.2f objects, want at most %v", got, bound)
	}
}
