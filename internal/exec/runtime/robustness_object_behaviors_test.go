package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// TestRuntimeRobustnessObjectBehaviors covers machines exhibited and actions performed by an
// object that cannot run.
func TestRuntimeRobustnessObjectBehaviors(t *testing.T) {
	t.Run("object_exhibited_machine_never_settles", testObjectExhibitedMachineNeverSettles)
	t.Run("object_exhibited_machine_without_an_initial_state", testObjectExhibitedMachineWithoutAnInitialState)
	t.Run("object_exhibited_machine_whose_only_edge_is_a_first_succession", testObjectExhibitedMachineWhoseOnlyEdgeIsAFirstSuccession)
	t.Run("object_exhibited_machine_attribute_write_violates_multiplicity", testObjectExhibitedMachineAttributeWriteViolatesMultiplicity)
	t.Run("object_performed_action_attribute_write_violates_multiplicity", testObjectPerformedActionAttributeWriteViolatesMultiplicity)
	t.Run("object_performed_action_occurrence_holds_a_non_object", testObjectPerformedActionOccurrenceHoldsANonObject)
}

// testObjectExhibitedMachineNeverSettles: a machine an object exhibits is bounded
// by the same budgets as one run on its own, so materializing an object whose
// machine never settles reports a budget error rather than spinning.
func testObjectExhibitedMachineNeverSettles(t *testing.T) {
	src := `
	package test {
		part def Spinner {
			attribute ticks : Integer = 0;
			exhibit state modes {
				entry; then spin;
				state spin {
					do action tick { assign ticks := ticks + 1; }
				}
				transition again first spin then spin;
			}
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Spinner")
	if err == nil {
		t.Fatal("expected a budget error for an exhibited machine that never settles")
	}
	if !strings.Contains(err.Error(), "exceeded max") && !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("error = %v, want a budget error", err)
	}
}

// testObjectExhibitedMachineWithoutAnInitialState: a machine stating states but no
// entry into them is reported when the object's execution of it initializes, with
// the behavior and the type named.
func testObjectExhibitedMachineWithoutAnInitialState(t *testing.T) {
	src := `
	package test {
		part def Controller {
			exhibit state modes {
				state off;
				state on;
			}
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Controller")
	if err == nil {
		t.Fatal("expected an error for a machine with no initial state")
	}
	if !strings.Contains(err.Error(), "modes") || !strings.Contains(err.Error(), "initial") {
		t.Errorf("error = %v, want one naming the machine and its missing initial state", err)
	}
}

// testObjectExhibitedMachineWhoseOnlyEdgeIsAFirstSuccession: `first b then c;`
// orders two states and enters neither, so a machine with no entry marker is
// reported as having no initial state rather than started in c.
func testObjectExhibitedMachineWhoseOnlyEdgeIsAFirstSuccession(t *testing.T) {
	src := `
	package test {
		part def Controller {
			exhibit state modes {
				state b;
				state c;
				first b then c;
			}
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Controller")
	if err == nil {
		t.Fatal("expected an error for a machine with no initial state")
	}
	if !strings.Contains(err.Error(), "modes") || !strings.Contains(err.Error(), "initial") {
		t.Errorf("error = %v, want one naming the machine and its missing initial state", err)
	}
}

// testObjectExhibitedMachineAttributeWriteViolatesMultiplicity checks that an
// occurrence write failure remains typed and terminates normally.
func testObjectExhibitedMachineAttributeWriteViolatesMultiplicity(t *testing.T) {
	src := `
	package test {
		state def Modes {
			attribute samples : Integer[2] nonunique = (0, 0);
			entry; then active;
			state active {
				entry action record {
					assign samples := 1;
				}
			}
		}
		part def Controller {
			exhibit state modes : Modes;
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Controller")
	if !errors.Is(err, errStatePerformanceOccurrence) {
		t.Fatalf("error = %v, want errStatePerformanceOccurrence", err)
	}
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("error = %v, want ErrMultiplicityViolation", err)
	}
}

// testObjectPerformedActionAttributeWriteViolatesMultiplicity checks that a
// performed action's occurrence write failure remains typed.
func testObjectPerformedActionAttributeWriteViolatesMultiplicity(t *testing.T) {
	src := `
	package test {
		action def Record {
			attribute samples : Integer[2] nonunique = (0, 0);
			action step {
				assign samples := 1;
			}
			first step;
		}
		part def Logger {
			perform action recording : Record;
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Logger")
	if !errors.Is(err, errActionPerformanceOccurrence) {
		t.Fatalf("error = %v, want errActionPerformanceOccurrence", err)
	}
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("error = %v, want ErrMultiplicityViolation", err)
	}
}

// testObjectPerformedActionOccurrenceHoldsANonObject: a perform usage whose
// feature holds a value that is not an occurrence is reported, not performed.
func testObjectPerformedActionOccurrenceHoldsANonObject(t *testing.T) {
	src := `
	package test {
		action def Bump {
			attribute count : Integer = 0;
			action step {
				assign count := count + 1;
			}
			first step;
		}
		part def Host {
			perform action work : Bump;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<performed-action-non-object>", parseAndBuild(t, src))
	inst, err := ctx.materialize(oneSymbol(t, idx, "test::Host"), 0, nil, "")
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	fv, ok := inst.FeatureValues["work"]
	if !ok {
		t.Fatal("object has no work feature")
	}
	fv.Value = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 5}}
	fv.Materialized = true

	err = ctx.startClassifierBehaviors(inst, 0)
	if !errors.Is(err, errActionPerformanceOccurrence) {
		t.Fatalf("error = %v, want errActionPerformanceOccurrence", err)
	}
	if !strings.Contains(err.Error(), "not an occurrence") {
		t.Errorf("error = %v, want one naming the value the feature holds", err)
	}
}
