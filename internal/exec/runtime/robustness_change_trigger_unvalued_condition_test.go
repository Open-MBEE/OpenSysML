package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A change condition reading an unvalued feature waits for the write that values it;
// one that fails to evaluate fails its machine, recorded, the object standing.
func TestRuntimeRobustnessChangeTriggerUnvaluedCondition(t *testing.T) {
	const relaySource = `package test {
		private import ScalarValues::*;
		part def Relay {
			attribute coilVoltage : Real;
			exhibit state contacts {
				entry; then open;
				state open;
				accept when coilVoltage >= 19.6 then closed;
				state closed;
			}
		}
		part relay : Relay;
	}`
	realValue := func(x float64) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: x}}
	}
	// A typed model reads the required attribute as uninitialized, a parsing
	// one as holding no value; both are reads of a feature with no value.
	modelForms := map[string]func(*semantics.Model, *resolve.Resolver) *Model{
		"typed": typedModel, "parsing": parsingModel,
	}
	relayOf := func(t *testing.T, form func(*semantics.Model, *resolve.Resolver) *Model) (*Context, *Instance, *StateExecutor) {
		t.Helper()
		idx := libs.NewModelIndex()
		idx.AddDocument("<test>", parseAndBuild(t, relaySource))
		idx.ExpandWildcardImports()
		resolver := resolve.New(idx)
		ctx := NewContext(form(semantics.NewModel(resolver), resolver), 10000)
		matches := idx.LookupQualified("test::relay")
		if len(matches) != 1 {
			t.Fatalf("relay: got %d matching symbols, want 1", len(matches))
		}
		relay, err := ctx.occurrenceOf(matches[0])
		if err != nil {
			t.Fatalf("occurrenceOf(relay): %v", err)
		}
		return ctx, relay, relay.ExhibitedStates()[0].State
	}

	for name, form := range modelForms {
		t.Run("a condition reading an unvalued feature waits and names it, "+name+" model", func(t *testing.T) {
			_, _, exec := relayOf(t, form)
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("RunToCompletion() = %v, want the machine to wait", err)
			}
			assertCurrentState(t, exec, "open")
			if exec.State() != StateSuspended {
				t.Fatalf("state = %v, want suspended", exec.State())
			}
			reason := exec.SuspendReason()
			if !strings.Contains(reason, "waiting on change condition") || !strings.Contains(reason, "condition has no result yet") || !strings.Contains(reason, "coilVoltage") {
				t.Errorf("reason = %q, want the unvalued feature the condition waits on", reason)
			}
		})

		t.Run("the write that gives the feature a value decides the condition, "+name+" model", func(t *testing.T) {
			ctx, relay, exec := relayOf(t, form)
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("initial run: %v", err)
			}
			if err := relay.SetFeatureValue(ctx, "coilVoltage", realValue(10.0)); err != nil {
				t.Fatalf("SetFeatureValue(10.0): %v", err)
			}
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("run after writing 10.0: %v", err)
			}
			assertCurrentState(t, exec, "open")
			if reason := exec.SuspendReason(); !strings.Contains(reason, "condition is false") {
				t.Errorf("reason = %q, want the condition reported false once it has a result", reason)
			}
			if err := relay.SetFeatureValue(ctx, "coilVoltage", realValue(24.0)); err != nil {
				t.Fatalf("SetFeatureValue(24.0): %v", err)
			}
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("run after writing 24.0: %v", err)
			}
			assertCurrentState(t, exec, "closed")
		})
	}

	t.Run("a condition that fails to evaluate is still an error", func(t *testing.T) {
		ctx, divider, err := instantiateWithLibraries(t, `package test {
			private import ScalarValues::*;
			part def Divider {
				attribute zero : Integer = 0;
				exhibit state m {
					entry; then waiting;
					state waiting;
					accept when 1 / zero > 0 then done;
					state done;
				}
			}
			part divider : Divider;
		}`, "test::divider")
		if !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("instantiate divider: %v, want the condition's division by zero recorded on the machine", err)
		}
		if _, live := ctx.Instance(divider.ID); !live {
			t.Fatal("the object whose change condition failed to evaluate is not held")
		}
		failed := ctx.StartFailures(divider)
		if len(failed) != 1 || failed[0].Name != "m" || failed[0].State == nil {
			t.Fatalf("failed behaviors = %v, want the machine m alone", failed)
		}
		if !errors.Is(failed[0].Err, ErrDivisionByZero) || !strings.Contains(failed[0].Err.Error(), "change condition") {
			t.Errorf("recorded failure = %v, want the change condition's ErrDivisionByZero", failed[0].Err)
		}
		if strings.Contains(failed[0].Err.Error(), "no result yet") {
			t.Errorf("recorded failure = %v, want an error, not a wait", failed[0].Err)
		}
		if !failed[0].completed() {
			t.Error("the failed machine is not ended")
		}
		notes := ctx.StartFailureNotes(divider)
		if len(notes) != 1 {
			t.Fatalf("notes = %v, want one for the failed machine", notes)
		}
		d := notes[0].Diagnostic()
		if d.Code != BehaviorStartFailureCode || !strings.Contains(d.Message, "state machine m") || !strings.Contains(d.Message, ErrDivisionByZero.Error()) {
			t.Errorf("diagnostic = %+v, want %s naming machine m and its division by zero", d, BehaviorStartFailureCode)
		}
	})
}
