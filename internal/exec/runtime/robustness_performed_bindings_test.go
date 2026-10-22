package runtime

import (
	"errors"
	"strings"
	"testing"
)

const performedBindingFixture = `package test {
	private import ScalarValues::*;
	item def Report { attribute size : Real; }
	action def Measure { out report : Report; }
	action def Decide {
		in report : Report;
		out go : Boolean = report.size <= 10.0;
	}
	action def Sequence {
		action measure : Measure;
		then action decision : Decide { in report = measure.report; }
		then done;
	}
	part context {
		item theReport : Report { attribute :>> size = 6.8; }
		perform action run : Sequence;
		bind run.measure.report = theReport;
		attribute go : Boolean = run.decision.go;
	}
	part unvalued {
		item theReport : Report;
		perform action run : Sequence;
		bind run.measure.report = theReport;
	}
	part mistyped {
		attribute theCount : Integer = 3;
		perform action run : Sequence;
		bind run.measure.report = theCount;
	}
}`

func TestRuntimeRobustnessPerformedBindings(t *testing.T) {
	t.Run("bound_pin_holds_the_performers_value", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedBindingFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, "test::context"))
		if err != nil {
			t.Fatalf("Instantiate context: %v", err)
		}
		fv, err := self.GetFeatureValue(ctx, "go")
		if err != nil {
			t.Fatalf("read go: %v", err)
		}
		if got := soleElement(fv.HeldValue()); !valueEqual(got, boolValue(true)) {
			t.Errorf("go = %v, want true", got)
		}
	})

	t.Run("unvalued_other_end_leaves_the_pin_empty", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedBindingFixture))
		_, err := ctx.Instantiate(oneSymbol(t, idx, "test::unvalued"))
		if err == nil {
			t.Fatal("Instantiate unvalued: err = nil, want the output default failing over an unbound report")
		}
		if !errors.Is(err, ErrTypeMismatch) && !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("Instantiate unvalued: err = %v, want a typed runtime error", err)
		}
	})

	t.Run("mistyped_other_end_is_refused_at_the_pin", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedBindingFixture))
		_, err := ctx.Instantiate(oneSymbol(t, idx, "test::mistyped"))
		if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "report") {
			t.Errorf("Instantiate mistyped: err = %v, want ErrTypeMismatch naming report", err)
		}
	})
}
