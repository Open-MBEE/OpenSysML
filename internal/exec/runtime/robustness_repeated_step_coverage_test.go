package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestRuntimeRobustnessRepeatedStepCoverage(t *testing.T) {
	// A bind at a repeated step's out-pin takes the one value every performance
	// agrees on; differing outputs are the conflict a binding cannot resolve.
	t.Run("out-pin-binding-conflict", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute c : Integer = 0;
				attribute r : Integer[0..1];
				first start then b;
				action b[2] {
					out y : Integer;
					assign c := c + 1;
					assign y := c;
				}
				bind b.y = r;
				succession first [*] b then [1] done;
			}
		}`)
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("execution error = %v, want ErrBindingConflict", err)
		}
	})

	// A bind at a repeated step's in-pin takes a single value for every
	// performance; a multi-valued end is a distribution the model leaves open.
	t.Run("in-pin-multi-valued-end", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute k : Integer[2] = (1, 2);
				first start then a;
				action a[2] { in x : Integer; }
				bind a.x = k;
				succession first [*] a then [1] done;
			}
		}`)
		if !errors.Is(err, ErrActionStepMultiplicity) {
			t.Fatalf("execution error = %v, want ErrActionStepMultiplicity", err)
		}
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) {
			t.Fatalf("execution error = %v, want *lower.StepMultiplicityError", err)
		}
		if stepErr.Code != lower.StepMultiplicityUnsupportedCode {
			t.Errorf("step error code = %q, want %q", stepErr.Code, lower.StepMultiplicityUnsupportedCode)
		}
		const reason = "a binding distributes a multi-valued end over the performances in an assignment the model leaves open"
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("execution error = %q, want reason %q", err, reason)
		}
	})

	// A read of a repeated step's pin before every performance has ended is the
	// error a read of a not-yet-performed step gives.
	t.Run("read-before-performed", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			private import CollectionFunctions::*;
			action def A {
				attribute n : Integer = 0;
				first start then q;
				action q { assign n := size(a.x); }
				succession first q then [*] a;
				action a[2] { out x : Integer = 1; }
				succession first [*] a then [1] done;
			}
		}`)
		if !errors.Is(err, ErrNodeNotPerformed) {
			t.Fatalf("execution error = %v, want ErrNodeNotPerformed", err)
		}
	})

	// Object flows at pins of a repeated step stay refused.
	t.Run("flow-at-repeated-pin", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				first start then a;
				action a[3] { out o : Integer = 1; }
				succession first [*] a then [1] b;
				action b { in i : Integer; }
				flow a.o to b.i;
				then done;
			}
		}`)
		if !errors.Is(err, ErrActionStepMultiplicity) {
			t.Fatalf("execution error = %v, want ErrActionStepMultiplicity", err)
		}
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) {
			t.Fatalf("execution error = %v, want *lower.StepMultiplicityError", err)
		}
		if stepErr.Code != lower.StepMultiplicityUnsupportedCode {
			t.Errorf("step error code = %q, want %q", stepErr.Code, lower.StepMultiplicityUnsupportedCode)
		}
	})

	// `perform action run[0]` enacts no performance; `run[2]` enacts two, each
	// adding one to the part's `count`.
	t.Run("perform-action-counts-on-part", func(t *testing.T) {
		for _, test := range []struct {
			multiplicity string
			want         int64
		}{
			{"[0]", 0},
			{"[2]", 2},
		} {
			t.Run("run"+test.multiplicity, func(t *testing.T) {
				file := parseAndBuild(t, `package test {
					private import ScalarValues::*;
					part def Host {
						attribute count : Integer = 0;
						perform action run`+test.multiplicity+` {
							action step { assign count := count + 1; }
							first step;
						}
					}
				}`)
				index, _, ctx := buildRuntimeWithLibraries(t, "<test>", file)
				symbol := findSymbolByName(index.DocumentRoot("<test>"), "Host", ast.DefPart)
				if symbol == nil {
					t.Fatal("part Host not found")
				}
				inst, err := ctx.Instantiate(symbol)
				if err != nil {
					t.Fatalf("Instantiate: %v", err)
				}
				if got := featureIntValue(t, ctx, inst, "count"); got != test.want {
					t.Errorf("count = %d, want %d", got, test.want)
				}
			})
		}
	})

	// A repeated step inside a while inside a for counts its performances once
	// per pass of the innermost body.
	t.Run("nested-while-in-for", func(t *testing.T) {
		outputs, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute c : Integer = 0;
				attribute j : Integer = 0;
				first start then worker;
				action worker {
					for i : Integer in (1, 2) {
						assign j := 0;
						while j < 2 {
							first start then a;
							action a[3] { assign c := c + 1; }
							succession first [*] a then [1] t;
							action t { assign j := j + 1; }
						}
					}
				}
				then done;
			}
		}`)
		if err != nil {
			t.Fatalf("executeActionSource: %v", err)
		}
		assertIntOutput(t, outputs, "c", 12)
	})

	// A non-fixed count stays refused inside a loop body, same as at action
	// level.
	t.Run("non-fixed-in-loop-body", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute i : Integer = 0;
				first start then worker;
				action worker {
					while i < 1 {
						first start then a;
						action a[0..2] { }
						assign i := i + 1;
					}
				}
				then done;
			}
		}`)
		if !errors.Is(err, ErrActionStepMultiplicity) {
			t.Fatalf("execution error = %v, want ErrActionStepMultiplicity", err)
		}
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) {
			t.Fatalf("execution error = %v, want *lower.StepMultiplicityError", err)
		}
		if stepErr.Code != lower.StepMultiplicityNotFixedCode {
			t.Errorf("step error code = %q, want %q", stepErr.Code, lower.StepMultiplicityNotFixedCode)
		}
	})

	// A nested repeated read yields the sequence over performances, which a
	// single-valued target cannot take.
	t.Run("repeated-read-into-single-valued", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute total : Integer = 0;
				first start then outer;
				action outer {
					first start then inner;
					action inner[3] { attribute x : Integer = 1; }
					then done;
				}
				succession first outer then q;
				action q { assign total := outer.inner.x; }
				then done;
			}
		}`)
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Fatalf("execution error = %v, want ErrMultiplicityViolation", err)
		}
	})

	// Explore agrees with run: both sibling performances of `a` interleave but
	// accrue to one outcome.
	t.Run("explore-pin-value", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			private import ScalarValues::*;
			private import NumericalFunctions::*;

			action def Inc {
				in x : Integer;
				out y : Integer;
				first start then bump;
				action bump { assign y := x + 1; }
				then done;
			}

			action def PinValue {
				attribute c : Integer = 4;
				attribute total : Integer = 0;
				first start then a;
				action a : Inc[2] { in x = c; }
				succession first [*] a then [1] q;
				action q { assign total := sum(a.y); }
				succession first q then done;
			}
		}`)
		x := m.exploreAction(t, "explore", "PinValue")
		if !x.Complete() {
			t.Fatalf("exploration incomplete: %s", x.Status())
		}
		if len(x.Outcomes) != 1 {
			t.Fatalf("outcomes %v, want exactly one", outcomeTexts(x))
		}
		outcome := x.Outcomes[0].Outcome
		if outcome.Err != nil {
			t.Fatalf("outcome error = %v", outcome.Err)
		}
		if got := intValue(t, outcome.Outputs, "total"); got != 10 {
			t.Errorf("total = %d, want 10", got)
		}
	})
}

// featureIntValue reads an integer-valued feature of an instance.
func featureIntValue(t *testing.T, ctx *Context, inst *Instance, name string) int64 {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s): %v", name, err)
	}
	value := fv.HeldValue()
	if value.Kind != ValConst || value.Const.Kind != semantics.ValInt {
		t.Fatalf("feature %q = %v, want an integer", name, value)
	}
	return value.Const.Int
}
