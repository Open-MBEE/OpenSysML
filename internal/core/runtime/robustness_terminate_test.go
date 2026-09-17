package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// TestRuntimeRobustnessTerminate exercises the shapes a `terminate` refuses: each is
// a typed error naming what could not be ended, never a silent no-op or a panic.
// A calculation's refusal of `terminate` stays with the shared cases.
func TestRuntimeRobustnessTerminate(t *testing.T) {
	t.Run("terminate_of_an_ended_performance", testTerminateOfAnEndedPerformance)
	t.Run("terminate_of_an_unknown_name", testTerminateOfAnUnknownName)
	t.Run("terminate_of_a_non_action_feature", testTerminateOfANonActionFeature)
	t.Run("terminate_of_a_literal_value", testTerminateOfALiteralValue)
	t.Run("terminate_of_a_qualified_occurrence", testTerminateOfAQualifiedOccurrence)
	t.Run("terminate_of_a_destroyed_occurrence", testTerminateOfADestroyedOccurrence)
	t.Run("terminate_of_a_part_reached_after_its_whole_ended", testTerminateOfAPartReachedAfterItsWholeEnded)
	t.Run("terminate_of_an_occurrence_expression", testTerminateOfAnOccurrenceExpression)
	t.Run("terminate_of_a_node_of_a_sibling_flow", testTerminateOfANodeOfASiblingFlow)
	t.Run("terminate_usage_stating_a_flow_of_its_own", testTerminateUsageStatingAFlowOfItsOwn)
	t.Run("terminate_of_an_unknown_name_in_a_state_body", testTerminateOfAnUnknownNameInAStateBody)
	t.Run("terminate_of_an_ended_occurrence_in_a_state_body", testTerminateOfAnEndedOccurrenceInAStateBody)
	t.Run("terminate_of_a_value_in_a_transition_effect", testTerminateOfAValueInATransitionEffect)
	t.Run("behavior_of_an_ended_performer", testBehaviorOfAnEndedPerformer)
}

// testTerminateOfAnEndedPerformance: `terminate c1;` after c1 completed names a
// performance that has already ended, which is reported rather than ended twice.
func testTerminateOfAnEndedPerformance(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		private import ScalarValues::*;
		action host {
			out attribute x : Integer = 0;
			first start;
			then action c1 { assign x := 1; }
			then action c2 { terminate c1; }
			then done;
		}
	}`)
	if !errors.Is(err, ErrPerformanceEnded) {
		t.Fatalf("error = %v, want ErrPerformanceEnded", err)
	}
}

// testTerminateOfAnUnknownName: a terminate naming nothing declared around it
// has no performance to end.
func testTerminateOfAnUnknownName(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		action host {
			first start;
			then action c1 { terminate nowhere; }
			then done;
		}
	}`)
	if !errors.Is(err, ErrTerminateTarget) {
		t.Fatalf("error = %v, want ErrTerminateTarget", err)
	}
}

// testTerminateUsageStatingAFlowOfItsOwn: a terminate action usage whose body states
// a flow of its own is refused at initialize, not run with the flow dropped.
func testTerminateUsageStatingAFlowOfItsOwn(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		private import ScalarValues::*;
		action host {
			out attribute x : Integer = 0;
			first start;
			then stop;
			action stop terminate {
				first start;
				then action inner { assign x := 1; }
				then done;
			}
			then done;
		}
	}`)
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
	}
}

// testTerminateOfANonActionFeature: a terminate naming a feature that is no action
// node names an occurrence, which an action body does not end yet.
func testTerminateOfANonActionFeature(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		private import ScalarValues::*;
		action host {
			out attribute x : Integer = 0;
			first start;
			then action c1 { terminate x; }
			then done;
		}
	}`)
	if !errors.Is(err, ErrTerminateOccurrence) {
		t.Fatalf("error = %v, want ErrTerminateOccurrence", err)
	}
}

// testTerminateOfAQualifiedOccurrence: a qualified name reaching an occurrence that is
// no action node ends that occurrence; naming it again is the occurrence refusal.
func testTerminateOfAQualifiedOccurrence(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		part def V;
		part victim : V;
		action host {
			first start;
			then action c1 { terminate test::victim; }
			then action c2 { terminate test::victim; }
			then done;
		}
	}`)
	if !errors.Is(err, ErrTerminateOccurrence) || !errors.Is(err, ErrOccurrenceLifetime) {
		t.Fatalf("error = %v, want ErrTerminateOccurrence wrapping ErrOccurrenceLifetime", err)
	}
}

// testTerminateOfAnOccurrenceExpression: a terminate whose target expression names
// no occurrence where it is stated — `this` in a body no object performs — ends nothing.
func testTerminateOfAnOccurrenceExpression(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		action host {
			first start;
			then action c1 { terminate this.self; }
			then done;
		}
	}`)
	if !errors.Is(err, ErrTerminateTarget) {
		t.Fatalf("error = %v, want ErrTerminateTarget", err)
	}
}

// testTerminateOfANodeOfASiblingFlow: a terminate names action nodes of the flows
// around it only; a node nested in a sibling branch is out of its reach.
func testTerminateOfANodeOfASiblingFlow(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		private import ScalarValues::*;
		action host {
			fork split;
			action slow {
				action inner {
					accept after 10 [s];
					assign x := 1;
				}
			}
			action killer { terminate inner; }
			join sync;
			attribute x : Integer = 0;
			succession first start then split;
			succession first split then slow;
			succession first split then killer;
			succession first slow then sync;
			succession first killer then sync;
			succession first sync then done;
		}
	}`)
	if !errors.Is(err, ErrTerminateTarget) {
		t.Fatalf("error = %v, want ErrTerminateTarget", err)
	}
}

// testTerminateOfALiteralValue: a terminate whose expression evaluates to a value
// that is no occurrence has nothing whose lifetime could end.
func testTerminateOfALiteralValue(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		action host {
			first start;
			then action c1 { terminate 3 + 4; }
			then done;
		}
	}`)
	if !errors.Is(err, ErrTerminateOccurrence) || !errors.Is(err, ErrNotAnOccurrence) {
		t.Fatalf("error = %v, want ErrTerminateOccurrence wrapping ErrNotAnOccurrence", err)
	}
}

// testTerminateOfADestroyedOccurrence: an occurrence `destroy` ended is refused as
// destroyed, not ended a second time.
func testTerminateOfADestroyedOccurrence(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import OccurrenceFunctions::*;
		part def V;
		action host {
			part victim : V;
			first start;
			then action c1 { assign victim := destroy(victim); }
			then action c2 { terminate victim; }
			then done;
		}
	}`))
	_, err := ctx.ExecuteAction(findSymbolByName(idx.DocumentRoot("<test>"), "host", ast.DefAction))
	if !errors.Is(err, ErrTerminateOccurrence) || !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Fatalf("error = %v, want ErrTerminateOccurrence wrapping ErrOccurrenceDestroyed", err)
	}
}

// testTerminateOfAPartReachedAfterItsWholeEnded: a part first read after `terminate`
// ended its whole ended with the whole, so terminating it is refused as ended already.
func testTerminateOfAPartReachedAfterItsWholeEnded(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def Engine { attribute rate : Integer = 60; }
		part def Body { part engine : Engine[0..1] { :>> rate = 50; } }
		part patient : Body;
		action host {
			out attribute rate : Integer = 0;
			first start;
			then action c1 { terminate patient; }
			then action c2 assign rate := patient.engine.rate;
			then action c3 { terminate patient.engine; }
			then done;
		}
	}`))
	_, err := ctx.ExecuteAction(findSymbolByName(idx.DocumentRoot("<test>"), "host", ast.DefAction))
	if !errors.Is(err, ErrTerminateOccurrence) || !errors.Is(err, ErrOccurrenceLifetime) {
		t.Fatalf("error = %v, want ErrTerminateOccurrence wrapping ErrOccurrenceLifetime", err)
	}
	whole, ok := ctx.OccurrenceLife(1)
	if !ok || whole.Ended == 0 {
		t.Fatalf("whole's life = %v, %v; want ended", whole, ok)
	}
	part, ok := ctx.OccurrenceLife(2)
	if !ok || part.Began != whole.Began || part.Ended != whole.Ended || part.Destroyed {
		t.Fatalf("part's life = %v, %v; want the whole's, %v", part, ok, whole)
	}
}

// testTerminateOfAnUnknownNameInAStateBody: a state's entry behavior terminating a
// name nothing declares is the same refusal an action body gives.
func testTerminateOfAnUnknownNameInAStateBody(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state def Machine {
			entry; then s;
			state s { entry { terminate nowhere; } }
		}
	}`)
	if !errors.Is(err, ErrTerminateTarget) {
		t.Fatalf("error = %v, want ErrTerminateTarget", err)
	}
}

// testTerminateOfAnEndedOccurrenceInAStateBody: a do behavior ending an occurrence
// that ended already is refused by the occurrence's lifetime, in the state body too.
func testTerminateOfAnEndedOccurrenceInAStateBody(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		part def V;
		part victim : V;
		state def Machine {
			entry; then s;
			state s {
				do action twice {
					first start;
					then action once { terminate test::victim; }
					then action again { terminate test::victim; }
					then done;
				}
			}
		}
	}`)
	if !errors.Is(err, ErrTerminateOccurrence) || !errors.Is(err, ErrOccurrenceLifetime) {
		t.Fatalf("error = %v, want ErrTerminateOccurrence wrapping ErrOccurrenceLifetime", err)
	}
}

// testTerminateOfAValueInATransitionEffect: a transition effect terminating an
// attribute names a value, which no lifetime ends.
func testTerminateOfAValueInATransitionEffect(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		attribute def Go;
		state def Machine {
			attribute n : Integer = 0;
			entry; then s;
			state s;
			transition first s accept Go do action { terminate n; } then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	_, _, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
	if !errors.Is(err, ErrTerminateOccurrence) || !errors.Is(err, ErrNotAnOccurrence) {
		t.Fatalf("error = %v, want ErrTerminateOccurrence wrapping ErrNotAnOccurrence", err)
	}
}

// testBehaviorOfAnEndedPerformer: an object a terminate ended performs nothing
// further; a behavior begun for it afterwards is refused at creation.
func testBehaviorOfAnEndedPerformer(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def Probe {
			attribute n : Integer = 0;
			action bump { assign n := n + 1; }
		}
		part p : Probe;
		action reaper { first start; then action kill { terminate test::p; } then done; }
	}`))
	p := idx.LookupQualified("test::p")
	if len(p) != 1 {
		t.Fatalf("test::p: %d matching symbols, want 1", len(p))
	}
	inst, err := ctx.Instantiate(p[0])
	if err != nil {
		t.Fatalf("Instantiate(p): %v", err)
	}
	if _, err := ctx.ExecuteAction(findSymbolByName(idx.DocumentRoot("<test>"), "reaper", ast.DefAction)); err != nil {
		t.Fatalf("reaper: %v", err)
	}
	bump := idx.LookupQualified("test::Probe::bump")
	if len(bump) != 1 {
		t.Fatalf("test::Probe::bump: %d matching symbols, want 1", len(bump))
	}
	_, err = ctx.CreateActionExecutorFor(bump[0], inst)
	if !errors.Is(err, ErrOccurrenceLifetime) {
		t.Fatalf("error = %v, want ErrOccurrenceLifetime", err)
	}
}
