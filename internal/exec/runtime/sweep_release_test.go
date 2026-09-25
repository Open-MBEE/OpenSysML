package runtime

import (
	"context"
	"testing"
)

// A released row keeps its outputs and trace but no context, so a caller reading
// numbers alone need not keep every run's object graph alive.
func TestSweepRowReleasesItsContextWhenAsked(t *testing.T) {
	ctx, scope := sweepFixture(t)
	sym := calcNamed(t, scope, "Twice")
	plan := resolvedPlan(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{rangeOf("n", intOf(1), intOf(3))},
	})
	for _, release := range []bool{false, true} {
		var contexts []*Context
		var traces []*TraceRecorder
		traced := func(row *Context) *Context {
			tr := NewTraceRecorder()
			row.SetTrace(tr)
			contexts, traces = append(contexts, row), append(traces, tr)
			return row
		}
		fresh := func(int) (*Context, error) { return traced(NewContext(ctx.model, ctx.maxSteps)), nil }
		run := func(rowCtx *Context, bindings []SweepBinding) (SweepRunResult, error) {
			result, err := sweepCalcRun(sym, scope)(rowCtx, bindings)
			result.ReleaseContext = release
			return result, err
		}
		workers := SweepWorkers{First: traced(NewContext(ctx.model, ctx.maxSteps)), Jobs: 1, Fresh: fresh}
		table, err := RunSweepWith(context.Background(), workers, "test::Twice", plan, 0, run)
		if err != nil {
			t.Fatalf("release=%v: sweep of Twice: %v", release, err)
		}
		if got, want := tableText(table), "n=1 result -> 2\nn=2 result -> 4\nn=3 result -> 6"; got != want {
			t.Errorf("release=%v: table is\n%s\nwant\n%s", release, got, want)
		}
		if len(traces) != len(table.Rows) {
			t.Fatalf("release=%v: %d contexts for %d rows", release, len(traces), len(table.Rows))
		}
		for i, row := range table.Rows {
			switch {
			case release && row.Context != nil:
				t.Errorf("release=%v: row %d keeps its context", release, i)
			case !release && row.Context != contexts[i]:
				t.Errorf("release=%v: row %d lost the context it ran in", release, i)
			}
			if row.Trace != traces[i] {
				t.Errorf("release=%v: row %d does not keep the trace its run recorded", release, i)
			}
		}
	}
}
