package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A transition guard reading a feature with no value is not true (SysML v2 §7.18.2): the
// machine runs on exactly as under a false guard, while a failing evaluation is still an error.
func TestRuntimeRobustnessTransitionGuardUnvalued(t *testing.T) {
	// controller declares v by decl and guards the transitions of machine on it.
	controller := func(decl, machine string) string {
		return `package test {
			private import ScalarValues::*;
			attribute def Go;
			part def Ctl {
				` + decl + `
				exhibit state sm {` + machine + `}
			}
			part ctl : Ctl;
		}`
	}
	const unvalued, falsy = "attribute v : Real;", "attribute v : Real = 0.0;"
	realValue := func(x float64) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: x}}
	}
	// A typed model reads the required attribute as uninitialized, a parsing
	// one as holding no value; both are reads of a feature with no value.
	modelForms := map[string]func(*semantics.Model, *resolve.Resolver) *Model{
		"typed": typedModel, "parsing": parsingModel,
	}
	machineOf := func(t *testing.T, form func(*semantics.Model, *resolve.Resolver) *Model, src string) (*Context, *Instance, *StateExecutor) {
		t.Helper()
		idx := libs.NewModelIndex()
		idx.AddDocument("<test>", parseAndBuild(t, src))
		idx.ExpandWildcardImports()
		resolver := resolve.New(idx)
		ctx := NewContext(form(semantics.NewModel(resolver), resolver), 10000)
		matches := idx.LookupQualified("test::ctl")
		if len(matches) != 1 {
			t.Fatalf("ctl: got %d matching symbols, want 1", len(matches))
		}
		ctl, err := ctx.occurrenceOf(matches[0])
		if err != nil {
			t.Fatalf("occurrenceOf(ctl): %v", err)
		}
		if failed := ctx.StartFailures(ctl); len(failed) != 0 {
			t.Fatalf("StartFailures = %v, want the machine to start", failed[0].Err)
		}
		return ctx, ctl, ctl.ExhibitedStates()[0].State
	}
	// A step writes v (when nonzero) and/or sends Go, then runs the machine on.
	type step struct {
		write float64
		send  bool
	}
	// statesAfter runs the machine of src through steps and returns the state
	// it rests in initially and after each step, suspended rather than failed.
	statesAfter := func(t *testing.T, form func(*semantics.Model, *resolve.Resolver) *Model, src string, steps []step) string {
		t.Helper()
		ctx, ctl, exec := machineOf(t, form, src)
		var states []string
		for i := 0; i <= len(steps); i++ {
			if i > 0 {
				if s := steps[i-1]; s.write != 0 {
					if err := ctl.SetFeatureValue(ctx, "v", realValue(s.write)); err != nil {
						t.Fatalf("step %d: SetFeatureValue(%v): %v", i, s.write, err)
					}
				} else if s.send {
					exec.SendSignal("Go", nil)
				}
			}
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("step %d: RunToCompletion() = %v, want the machine to keep running", i, err)
			}
			if exec.State() != StateSuspended {
				t.Fatalf("step %d: state = %v, want suspended", i, exec.State())
			}
			states = append(states, StateVertexName(exec.CurrentState()))
		}
		return strings.Join(states, " ")
	}

	cases := []struct {
		name, machine string
		steps         []step
		want          string
	}{{
		// The write alone moves nothing, as for a false guard; the Go re-entering s1 has its completion read the guard again.
		"completion guard", `
			entry; then s1;
			state s1;
			transition first s1 if v > 1.0 then s2;
			transition first s1 accept Go then s1;
			state s2;`,
		[]step{{send: true}, {write: 2.0}, {send: true}}, "s1 s1 s1 s2",
	}, {
		// A Go under a guard with no result is consumed without firing, as under a false one.
		"triggered guard", `
			entry; then s1;
			state s1;
			transition first s1 accept Go if v > 1.0 then s2;
			state s2;`,
		[]step{{send: true}, {write: 0.5}, {send: true}, {write: 2.0}, {send: true}}, "s1 s1 s1 s1 s1 s2",
	}, {
		// The entry transition whose guard has no result yields to the default.
		"entry transition guard", `
			entry; if v > 1.0 then s1; then s2;
			state s1;
			state s2;`,
		nil, "s2",
	}}
	for name, form := range modelForms {
		for _, c := range cases {
			t.Run("an unvalued "+c.name+" is not enabled, as a false one is not, "+name+" model", func(t *testing.T) {
				got := statesAfter(t, form, controller(unvalued, c.machine), c.steps)
				if got != c.want {
					t.Errorf("unvalued guard: states = %q, want %q", got, c.want)
				}
				if same := statesAfter(t, form, controller(falsy, c.machine), c.steps); same != got {
					t.Errorf("false guard: states = %q, unvalued guard: %q, want the same", same, got)
				}
			})
		}

		t.Run("a change transition whose guard has no result waits naming the feature, "+name+" model", func(t *testing.T) {
			_, _, exec := machineOf(t, form, controller(unvalued+" attribute armed : Boolean = true;", `
				entry; then s1;
				state s1;
				transition first s1 accept when armed if v > 1.0 then s2;
				state s2;`))
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("RunToCompletion() = %v, want the machine to wait", err)
			}
			assertCurrentState(t, exec, "s1")
			reason := exec.SuspendReason()
			if !strings.Contains(reason, "guard has no result yet") || !strings.Contains(reason, "v") {
				t.Errorf("reason = %q, want the unvalued feature the guard read", reason)
			}
		})
	}

	t.Run("a conditional succession whose guard has no result is not asserted", func(t *testing.T) {
		chooser := func(decl, guard, tail string) string {
			return `package test {
				private import ScalarValues::*;
				action outer {
					` + decl + `
					first start;
					then a;
					action a;
					first a if ` + guard + ` then b;
					` + tail + `
					action b;
					action c;
				}
			}`
		}
		for _, tail := range []string{"else c;", ""} {
			if err := runOuterAction(t, chooser(unvalued, "v > 1.0", tail)); err != nil {
				t.Errorf("unvalued guard with %q: RunToCompletion() = %v, want the run to complete as under a false guard", tail, err)
			}
		}
		err := runOuterAction(t, chooser("attribute z : Real = 0.0;", "1.0 / z > 1.0", "else c;"))
		if !errors.Is(err, ErrDivisionByZero) || !strings.Contains(err.Error(), "guard of node a") {
			t.Errorf("RunToCompletion() = %v, want the guard's ErrDivisionByZero", err)
		}
	})

	t.Run("a guard that fails to evaluate is still an error", func(t *testing.T) {
		for _, c := range []struct {
			name, decl, guard string
			want              error
		}{
			{"division by zero", "attribute z : Real = 0.0;", "1.0 / z > 1.0", ErrDivisionByZero},
			{"non-Boolean result", "attribute v : Real = 2.0;", "v", ErrTypeMismatch},
		} {
			ctx, ctl, err := instantiateWithLibraries(t, controller(c.decl, `
				entry; then s1;
				state s1;
				transition first s1 if `+c.guard+` then s2;
				state s2;`), "test::ctl")
			if !errors.Is(err, c.want) {
				t.Fatalf("%s: instantiate ctl: %v, want %v recorded on the machine", c.name, err, c.want)
			}
			if _, live := ctx.Instance(ctl.ID); !live {
				t.Fatalf("%s: the object whose guard failed to evaluate is not held", c.name)
			}
			failed := ctx.StartFailures(ctl)
			if len(failed) != 1 || failed[0].Name != "sm" || !errors.Is(failed[0].Err, c.want) {
				t.Fatalf("%s: failed behaviors = %v, want the machine sm with %v", c.name, failed, c.want)
			}
			if msg := failed[0].Err.Error(); !strings.Contains(msg, "guard of transition s1 -> s2") || strings.Contains(msg, "no result") {
				t.Errorf("%s: recorded failure = %q, want the guard's error, not a wait", c.name, msg)
			}
		}
	})
}
