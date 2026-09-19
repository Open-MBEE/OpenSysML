package lower

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// semanticStateSpaceModel answers lowering's questions from a semantics model
// over the bundled libraries, as the runtime does.
type semanticStateSpaceModel struct {
	idx   *symbols.Index
	model *semantics.Model
}

func (m semanticStateSpaceModel) LibrarySymbol(fqn string) *symbols.Symbol {
	for _, sym := range m.idx.LookupQualified(fqn) {
		if m.idx.Library(sym) {
			return sym
		}
	}
	return nil
}
func (m semanticStateSpaceModel) Specializes(sym, general *symbols.Symbol) bool {
	return m.model.Conforms(sym, general)
}
func (m semanticStateSpaceModel) MembersOf(sym *symbols.Symbol) []*symbols.Symbol {
	return m.model.MembersOf(sym)
}
func (m semanticStateSpaceModel) FeatureTypes(sym *symbols.Symbol) []*symbols.Symbol {
	return m.model.FeatureTypes(sym)
}
func (m semanticStateSpaceModel) ParameterDefault(sym *symbols.Symbol) (ast.Node, *symbols.Scope) {
	return m.model.ParameterDefault(sym)
}
func (m semanticStateSpaceModel) LibraryDeclared(sym *symbols.Symbol) bool { return m.idx.Library(sym) }

// stateSpaceAction parses src beside the libraries and finds action test::<name>,
// with its body scope and a model over the whole.
func stateSpaceAction(t *testing.T, src, name string) (*symbols.Symbol, *symbols.Scope, StateSpaceModel) {
	t.Helper()
	file := parser.New(source.New("<test>", []byte(src))).ParseFile()
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", file)
	idx.ExpandWildcardImports()
	model := semantics.NewModel(resolve.New(idx))
	pkg := idx.DocumentRoot("<test>").Children()[0]
	sym, _ := pkg.LookupLocal(name)
	if sym == nil {
		t.Fatalf("action %s not found", name)
	}
	scope := sym.Scope
	if scope == nil {
		scope = sym.OwnerScope
	}
	return sym, scope, semanticStateSpaceModel{idx: idx, model: model}
}

const stateSpacePrelude = `
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;
`

// A user's action specializing ContinuousStateSpaceDynamics lowers to the calcs it
// provides, the integrator it binds, and the crossing it declares; one specializing
// DiscreteStateSpaceDynamics lowers to its difference.
func TestStateSpaceDynamicsLowering(t *testing.T) {
	src := `package test {` + stateSpacePrelude + `
		attribute def FallState :> StateSpace;
		action def Touchdown :> ZeroCrossing;

		action fall : ContinuousStateSpaceDynamics, FixedStepDynamics {
			in :>> input = VectorOf((10.0));
			:>> stateSpace : FallState = VectorOf((19.0, 0.0));
			:>> timeStep = 0.5 [s];
			:>> stopTime = 10 [s];
			event occurrence touchdown : Touchdown {
				:>> guard = stateSpace.elements#(1);
				:>> terminal = true;
			}
			calc :>> getDerivative {
				in input : Input;
				in stateSpace : FallState;
				return : StateDerivative = VectorOf((stateSpace.elements#(2), 0.0 - input.elements#(1))) / 1 [s];
			}
			calc :>> getOutput {
				in input : Input;
				in stateSpace : FallState;
				return : Output = VectorOf((stateSpace.elements#(1)));
			}
			calc :>> getNextState {
				calc :>> integrate : Euler;
			}
		}

		action account : DiscreteStateSpaceDynamics, FixedStepDynamics {
			:>> stateSpace = VectorOf((1000.0));
			:>> timeStep = 1 [s];
			calc :>> getDifference {
				in input : Input;
				in stateSpace : StateSpace;
				return : StateSpace = VectorOf((0.05 * stateSpace.elements#(1)));
			}
			calc :>> getOutput {
				in input : Input;
				in stateSpace : StateSpace;
				return : Output = stateSpace;
			}
		}

		action plain { action a; }
	}`

	fall, scope, model := stateSpaceAction(t, src, "fall")
	if kind := StateSpaceKindOf(fall, model); kind != ContinuousDynamics {
		t.Fatalf("kind of fall = %v, want ContinuousDynamics", kind)
	}
	dyn, err := ToStateSpaceDynamics(fall, scope, model)
	if err != nil {
		t.Fatalf("ToStateSpaceDynamics(fall): %v", err)
	}
	if dyn.Action != fall || dyn.Scope != scope {
		t.Error("the lowering does not carry the action and its body scope")
	}
	for name, sym := range map[string]*symbols.Symbol{
		"stateSpace": dyn.State, "input": dyn.Input, "output": dyn.Output,
		"timeStep": dyn.TimeStep, "stopTime": dyn.StopTime, "time": dyn.Time,
		"getDerivative": dyn.Derivative, "getOutput": dyn.OutputCalc,
	} {
		if sym == nil || sym.Name != name {
			t.Errorf("%s lowered to %v, want the member of that name", name, sym)
		}
	}
	if model.LibraryDeclared(dyn.Derivative) || model.LibraryDeclared(dyn.OutputCalc) {
		t.Error("the calcs lowered are the library's abstract ones, not the model's")
	}
	if !model.LibraryDeclared(dyn.Time) {
		t.Error("time, which the model leaves to FixedStepDynamics, is not the library's")
	}
	if dyn.Difference != nil || dyn.NextState != nil {
		t.Errorf("continuous dynamics lowered a difference %v or a bodied next state %v", dyn.Difference, dyn.NextState)
	}
	if dyn.Integrator != IntegratorEuler || dyn.IntegratorType == nil || dyn.IntegratorType.Name != "Euler" {
		t.Errorf("integrator = %v (%v), want Euler", dyn.Integrator, dyn.IntegratorType)
	}
	if len(dyn.Crossings) != 1 {
		t.Fatalf("crossings = %d, want the one touchdown", len(dyn.Crossings))
	}
	crossing := dyn.Crossings[0]
	if crossing.Name != "touchdown" || crossing.Event == nil || crossing.Event.Name != "touchdown" {
		t.Errorf("crossing = %+v, want touchdown", crossing)
	}
	if crossing.EventType == nil || crossing.EventType.Name != "Touchdown" {
		t.Errorf("crossing type = %v, want Touchdown", crossing.EventType)
	}
	if crossing.Guard == nil || crossing.GuardScope == nil {
		t.Error("the crossing's guard or its scope was not lowered")
	}
	if crossing.Terminal == nil || crossing.TerminalScope == nil {
		t.Error("the crossing's terminal or its scope was not lowered")
	}

	account, scope, model := stateSpaceAction(t, src, "account")
	if kind := StateSpaceKindOf(account, model); kind != DiscreteDynamics {
		t.Fatalf("kind of account = %v, want DiscreteDynamics", kind)
	}
	dyn, err = ToStateSpaceDynamics(account, scope, model)
	if err != nil {
		t.Fatalf("ToStateSpaceDynamics(account): %v", err)
	}
	if dyn.Difference == nil || dyn.Difference.Name != "getDifference" || model.LibraryDeclared(dyn.Difference) {
		t.Errorf("difference = %v, want the model's getDifference", dyn.Difference)
	}
	if dyn.Derivative != nil || dyn.Integrator != IntegratorUnstated || len(dyn.Crossings) != 0 {
		t.Errorf("discrete dynamics lowered a derivative %v, integrator %v or crossings %v",
			dyn.Derivative, dyn.Integrator, dyn.Crossings)
	}
	if dyn.StopTime == nil || dyn.Input == nil {
		t.Error("stopTime and input, inherited unbound, are still carried so the runner knows them")
	}

	plain, scope, model := stateSpaceAction(t, src, "plain")
	if kind := StateSpaceKindOf(plain, model); kind != NotStateSpace {
		t.Errorf("kind of plain = %v, want NotStateSpace", kind)
	}
	if _, err := ToStateSpaceDynamics(plain, scope, model); !errors.Is(err, ErrUnsupportedStateSpace) {
		t.Errorf("ToStateSpaceDynamics(plain) = %v, want ErrUnsupportedStateSpace", err)
	}
}

// A dynamics leaving the run's integrator unstated lowers as such, and one whose
// getNextState the model bodies itself is carried to be called as written, even
// where that body also binds integrate.
func TestStateSpaceDynamicsIntegratorChoice(t *testing.T) {
	src := `package test {` + stateSpacePrelude + `
		calc def Heun :> Integrate;

		action unstated : ContinuousStateSpaceDynamics, FixedStepDynamics {
			:>> stateSpace = VectorOf((1.0));
			:>> timeStep = 0.1 [s];
			calc :>> getDerivative {
				in input : Input;
				in stateSpace : StateSpace;
				return : StateDerivative = stateSpace / 1 [s];
			}
			calc :>> getOutput {
				in input : Input;
				in stateSpace : StateSpace;
				return : Output = stateSpace;
			}
		}

		action rk4 :> unstated {
			calc :>> getNextState {
				calc :>> integrate : RK4;
			}
		}

		action heun :> unstated {
			calc :>> getNextState {
				calc :>> integrate : Heun;
			}
		}

		action bodied :> unstated {
			calc :>> getNextState {
				in input : Input;
				in stateSpace : StateSpace;
				in timeStep : DurationValue;
				return : StateSpace = stateSpace;
			}
		}

		action bodiedEuler :> unstated {
			calc :>> getNextState {
				in input : Input;
				in stateSpace : StateSpace;
				in timeStep : DurationValue;
				calc :>> integrate : Euler;
				return : StateSpace = stateSpace * 2.0;
			}
		}

		action redeclared :> unstated {
			calc :>> getNextState {
				in input : Input;
				in stateSpace : StateSpace;
			}
		}
	}`

	for name, want := range map[string]Integrator{"unstated": IntegratorUnstated, "rk4": IntegratorRK4, "redeclared": IntegratorUnstated} {
		action, scope, model := stateSpaceAction(t, src, name)
		dyn, err := ToStateSpaceDynamics(action, scope, model)
		if err != nil {
			t.Fatalf("ToStateSpaceDynamics(%s): %v", name, err)
		}
		if dyn.Integrator != want {
			t.Errorf("integrator of %s = %v, want %v", name, dyn.Integrator, want)
		}
		if dyn.NextState != nil {
			t.Errorf("%s lowered a bodied getNextState %v; the library's runs natively", name, dyn.NextState)
		}
	}

	heun, scope, model := stateSpaceAction(t, src, "heun")
	_, err := ToStateSpaceDynamics(heun, scope, model)
	if !errors.Is(err, ErrUnsupportedStateSpace) || !strings.Contains(err.Error(), "Heun") {
		t.Errorf("ToStateSpaceDynamics(heun) = %v, want ErrUnsupportedStateSpace naming Heun", err)
	}

	for _, name := range []string{"bodied", "bodiedEuler"} {
		bodied, scope, model := stateSpaceAction(t, src, name)
		dyn, err := ToStateSpaceDynamics(bodied, scope, model)
		if err != nil {
			t.Fatalf("ToStateSpaceDynamics(%s): %v", name, err)
		}
		if dyn.NextState == nil || dyn.NextState.Name != "getNextState" || model.LibraryDeclared(dyn.NextState) {
			t.Errorf("next state of %s = %v, want the model's bodied getNextState", name, dyn.NextState)
		}
		if dyn.Integrator != IntegratorUnstated {
			t.Errorf("integrator of %s = %v; a bodied getNextState runs as written", name, dyn.Integrator)
		}
	}
}

// A protocol calc the model leaves to the library's abstract declaration, and a
// crossing binding no guard, are refused when lowered, each naming what is missing.
func TestStateSpaceDynamicsUnsupportedShapes(t *testing.T) {
	src := `package test {` + stateSpacePrelude + `
		action noDerivative : ContinuousStateSpaceDynamics, FixedStepDynamics {
			:>> stateSpace = VectorOf((1.0));
			:>> timeStep = 0.1 [s];
			calc :>> getOutput {
				in input : Input;
				in stateSpace : StateSpace;
				return : Output = stateSpace;
			}
		}

		action noOutput : DiscreteStateSpaceDynamics, FixedStepDynamics {
			:>> stateSpace = VectorOf((1.0));
			:>> timeStep = 1 [s];
			calc :>> getDifference {
				in input : Input;
				in stateSpace : StateSpace;
				return : StateSpace = stateSpace;
			}
		}

		action noGuard : ContinuousStateSpaceDynamics, FixedStepDynamics {
			:>> stateSpace = VectorOf((1.0));
			:>> timeStep = 0.1 [s];
			event occurrence low : ZeroCrossing;
			calc :>> getDerivative {
				in input : Input;
				in stateSpace : StateSpace;
				return : StateDerivative = stateSpace / 1 [s];
			}
			calc :>> getOutput {
				in input : Input;
				in stateSpace : StateSpace;
				return : Output = stateSpace;
			}
		}
	}`

	for name, says := range map[string]string{
		"noDerivative": "leaves getDerivative abstract",
		"noOutput":     "leaves getOutput abstract",
		"noGuard":      "zero crossing low of action noGuard binds no guard",
	} {
		action, scope, model := stateSpaceAction(t, src, name)
		_, err := ToStateSpaceDynamics(action, scope, model)
		if !errors.Is(err, ErrUnsupportedStateSpace) {
			t.Errorf("ToStateSpaceDynamics(%s) = %v, want ErrUnsupportedStateSpace", name, err)
			continue
		}
		if !strings.Contains(err.Error(), says) {
			t.Errorf("ToStateSpaceDynamics(%s) = %q, want it to say %q", name, err, says)
		}
	}
}
