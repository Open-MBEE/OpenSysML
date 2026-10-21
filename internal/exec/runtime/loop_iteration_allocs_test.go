package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// loopIterationAllocs is what one more pass of the Loop calc's `while` costs in
// allocations: the difference between runs of 2n and n iterations, per iteration.
func loopIterationAllocs(t *testing.T, n int64) float64 {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", parser.New(source.New("<test>", []byte(integerBenchModel))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 1_000_000)
	loop := idx.LookupQualified("bench::Loop")[0]
	scope := idx.LookupQualified("bench")[0].Scope
	run := func(iterations int64) float64 {
		return testing.AllocsPerRun(10, func() {
			if _, err := ctx.InvokeCalc(loop, []Value{constInt(iterations), constInt(0)}, scope); err != nil {
				t.Fatal(err)
			}
		})
	}
	return (run(2*n) - run(n)) / float64(n)
}

// A pass of a `while` loop costs its body and the loop's bookkeeping, not a
// rebuild of what its condition reads: that footprint is the statement's,
// computed once per model, so a pass costs no more than the bound below.
func TestLoopIterationDoesNotRebuildConditionFootprint(t *testing.T) {
	const bound = 9
	got := loopIterationAllocs(t, 200)
	t.Logf("one more pass allocates %.2f objects", got)
	if got > bound {
		t.Errorf("one more pass of the loop allocates %.2f objects, want at most %d", got, bound)
	}
}
