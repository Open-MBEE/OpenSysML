package runtime

import (
	"errors"
	"testing"
)

const stepParameterFixture = `package test {
	private import ScalarValues::*;
	part def Gauge { attribute limit : Real; }
	action def Decide {
		in gauge : Gauge;
		out go : Boolean = gauge.limit <= 10.0;
	}
	action def Sequence {
		in system : Gauge;
		action decision : Decide { in gauge = system; }
		then done;
	}
	part context {
		part theGauge : Gauge { attribute :>> limit = 6.8; }
		perform action run : Sequence { in system = theGauge; }
		attribute go : Boolean = run.decision.go;
	}
	part explicit {
		part theGauge : Gauge { attribute :>> limit = 12.5; }
		perform action run : Sequence { in :>> system = theGauge; }
		attribute go : Boolean = run.decision.go;
	}
	part unsupplied {
		perform action run : Sequence;
		attribute go : Boolean = run.decision.go;
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

func TestRuntimeRobustnessStepParameter(t *testing.T) {
	t.Run("nested_pin_reads_the_steps_parameter_at_model_level", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, stepParameterFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, "test::context"))
		if err != nil {
			t.Fatalf("Instantiate context: %v", err)
		}
		run := instanceAtPath(t, ctx, self, "run")
		fv, err := run.GetFeatureValue(ctx, "system")
		if err != nil {
			t.Fatalf("read run.system: %v", err)
		}
		if system := soleElement(fv.HeldValue()); system.Kind != ValInstance {
			t.Fatalf("run.system = %s, want the gauge", FormatValue(system))
		}
		decision := instanceAtPath(t, ctx, self, "run.decision")
		fv, err = decision.GetFeatureValue(ctx, "gauge")
		if err != nil {
			t.Fatalf("read run.decision.gauge: %v", err)
		}
		if gauge := soleElement(fv.HeldValue()); gauge.Kind != ValInstance {
			t.Errorf("run.decision.gauge = %s, want the gauge run.system holds", FormatValue(gauge))
		}
		if got := readGo(t, ctx, self); !valueEqual(got, boolValue(true)) {
			t.Errorf("go = %v, want true", got)
		}
	})

	t.Run("explicit_redefinition_reads_the_same", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, stepParameterFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, "test::explicit"))
		if err != nil {
			t.Fatalf("Instantiate explicit: %v", err)
		}
		if got := readGo(t, ctx, self); !valueEqual(got, boolValue(false)) {
			t.Errorf("go = %v, want false", got)
		}
	})

	t.Run("unsupplied_step_parameter_is_a_typed_error", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, stepParameterFixture))
		_, err := ctx.Instantiate(oneSymbol(t, idx, "test::unsupplied"))
		if err == nil {
			t.Fatal("Instantiate unsupplied: err = nil, want the output default failing over an unbound gauge")
		}
		if !errors.Is(err, ErrTypeMismatch) && !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("Instantiate unsupplied: err = %v, want a typed runtime error", err)
		}
	})
}
