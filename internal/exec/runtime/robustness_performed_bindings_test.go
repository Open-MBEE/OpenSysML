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
	part zeroLinked {
		item theReport : Report { attribute :>> size = 6.8; }
		perform action run : Sequence;
		binding [0] bind run.measure.report = theReport;
	}
	action def Pair {
		action source : Measure;
		then action target : Decide;
		then done;
	}
	part pair {
		item theReport : Report { attribute :>> size = 6.8; }
		perform action run : Pair;
		bind run.source.report = theReport;
		bind run.target.report = run.source.report;
		attribute go : Boolean = run.target.go;
	}
	part def Holder { item value : Report; }
	part site {
		part near {
			part holder : Holder { item :>> value { attribute :>> size = 12.5; } }
			perform action run : Sequence;
			attribute go : Boolean = run.decision.go;
		}
		part holder : Holder { item :>> value { attribute :>> size = 6.8; } }
		bind near.run.measure.report = holder.value;
		attribute go : Boolean = near.go;
	}
	part def Flag { attribute verdict : Boolean; }
	part flagged {
		part near {
			part flag : Flag;
			item theReport : Report { attribute :>> size = 6.8; }
			perform action run : Sequence;
			bind run.measure.report = theReport;
		}
		part flag : Flag;
		bind near.run.decision.go = flag.verdict;
	}
}`

func readGo(t *testing.T, ctx *Context, self *Instance) Value {
	t.Helper()
	fv, err := self.GetFeatureValue(ctx, "go")
	if err != nil {
		t.Fatalf("read go of object #%d: %v", self.ID, err)
	}
	return soleElement(fv.HeldValue())
}

func TestRuntimeRobustnessPerformedBindings(t *testing.T) {
	t.Run("bound_pin_holds_the_performers_value", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedBindingFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, "test::context"))
		if err != nil {
			t.Fatalf("Instantiate context: %v", err)
		}
		if got := readGo(t, ctx, self); !valueEqual(got, boolValue(true)) {
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

	t.Run("zero_link_binding_supplies_nothing", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedBindingFixture))
		_, err := ctx.Instantiate(oneSymbol(t, idx, "test::zeroLinked"))
		if err == nil {
			t.Fatal("Instantiate zeroLinked: err = nil, want the output default failing over an unbound report")
		}
		if !errors.Is(err, ErrTypeMismatch) && !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("Instantiate zeroLinked: err = %v, want a typed runtime error", err)
		}
	})

	t.Run("binding_between_two_performed_nodes", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedBindingFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, "test::pair"))
		if err != nil {
			t.Fatalf("Instantiate pair: %v", err)
		}
		if got := readGo(t, ctx, self); !valueEqual(got, boolValue(true)) {
			t.Errorf("pair.go = %v, want true", got)
		}
	})

	t.Run("owners_chained_other_end_reads_on_the_owner", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedBindingFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, "test::site"))
		if err != nil {
			t.Fatalf("Instantiate site: %v", err)
		}
		if got := readGo(t, ctx, self); !valueEqual(got, boolValue(true)) {
			t.Errorf("site.go = %v, want true", got)
		}
	})

	t.Run("owners_chained_other_end_is_written_on_the_owner", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedBindingFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, "test::flagged"))
		if err != nil {
			t.Fatalf("Instantiate flagged: %v", err)
		}
		inner := instanceAtPath(t, ctx, self, "near.flag")
		if value := inner.FeatureValues["verdict"].HeldValue(); value.Kind != ValInvalid {
			t.Errorf("near.flag.verdict = %s, want nothing: the owner's flag is the bound end", FormatValue(value))
		}
		outer := instanceAtPath(t, ctx, self, "flag")
		if value := outer.FeatureValues["verdict"].HeldValue(); !valueEqual(soleElement(value), boolValue(true)) {
			t.Errorf("flag.verdict = %s, want true", FormatValue(value))
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
