package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessCaseBindings covers the frame a case's conditions read its
// run's bindings through: a case-local the run left declared without a value
// shadows the binding of the same name, so a condition naming it is undecided for
// the missing value rather than read through to the enclosing one.
func TestRuntimeRobustnessCaseBindings(t *testing.T) {
	t.Run("a mark on a frame holding no unvalued names is kept", func(t *testing.T) {
		var f frame
		f.markUnvalued("x")
		if !f.has("x") {
			t.Fatal("frame does not declare x after markUnvalued")
		}
		if _, ok, err := f.read(&Context{}, "x"); ok || !errors.Is(err, ErrNoValue) {
			t.Fatalf("read of unvalued x = %v, %v, want ErrNoValue", ok, err)
		}
	})

	t.Run("an unvalued case-local shadowing a bound parameter is undecided, not read through", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			part def Ship { attribute cost : Real = 5.0; }
			part ship : Ship;
			analysis bounded {
				subject s = ship;
				in limit : Real = 10.0;
				objective fits { require constraint { c < limit } }
				out c : Real = s.cost;
			}
		}`
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
		sym := oneSymbol(t, idx, "test::bounded")
		run, err := ctx.calcUsageRun(NewEvalContextIn(ctx, nil, nil), sym)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := run.outputValues(ctx); err != nil {
			t.Fatal(err)
		}
		if value, ok := run.env.lookup("limit"); !ok || FormatValue(value) != "10.0" {
			t.Fatalf("limit bound to %s, %v; want 10.0", FormatValue(value), ok)
		}
		// The body left limit declared and unvalued, as a constraint body's
		// performance leaves a root declaration no step wrote.
		run.bodyFrames = append(run.bodyFrames, frame{unvalued: map[string]bool{"limit": true}})

		bindings := run.bindingsFrame(ctx)
		if !bindings.has("limit") {
			t.Error("bindings frame does not declare the unvalued limit")
		}
		if _, ok, err := bindings.read(ctx, "limit"); ok || !errors.Is(err, ErrNoValue) {
			t.Errorf("read of unvalued limit = %v, %v, want ErrNoValue", ok, err)
		}

		verdicts := ctx.analysisVerdicts(run, sym, nil)
		if len(verdicts) != 1 || verdicts[0].Name != "fits" {
			t.Fatalf("verdicts = %+v, want the one objective fits", verdicts)
		}
		if got := verdicts[0]; got.Status != VerdictUndecided || !strings.Contains(got.Detail, "no value for feature limit") {
			t.Fatalf("objective fits = %s (%q), want undecided for the unvalued limit", got.Status, got.Detail)
		}
	})
}
