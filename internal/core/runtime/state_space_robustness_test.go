package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestStateSpaceRobustness exercises the shapes a state-space run refuses: each is
// a typed error naming the action and the protocol member at fault, never a wrong result.
func TestStateSpaceRobustness(t *testing.T) {
	t.Run("state_that_is_not_a_vector", testStateSpaceStateNotAVector)
	t.Run("state_without_an_initial_value", testStateSpaceStateWithoutAnInitialValue)
	t.Run("derivative_left_abstract", testStateSpaceDerivativeLeftAbstract)
	t.Run("difference_left_abstract", testStateSpaceDifferenceLeftAbstract)
	t.Run("derivative_that_is_not_a_vector", testStateSpaceDerivativeNotAVector)
	t.Run("output_that_is_not_a_vector", testStateSpaceOutputNotAVector)
	t.Run("step_not_stated", testStateSpaceStepNotStated)
	t.Run("step_that_is_zero", testStateSpaceStepZero)
	t.Run("step_that_is_negative", testStateSpaceStepNegative)
	t.Run("integrator_the_runtime_does_not_provide", testStateSpaceUnknownIntegrator)
	t.Run("divergent_state", testStateSpaceDivergentState)
	t.Run("step_past_the_last_instant", testStateSpaceStepPastLastInstant)
	t.Run("guard_that_is_not_a_number", testStateSpaceGuardNotANumber)
	t.Run("return_parameter", testStateSpaceReturnParameter)
}

// stateSpaceSource wraps an action body in a package importing what a
// state-space model needs, the action named `dyn`.
func stateSpaceSource(header, body string) string {
	return `package test {
		private import ScalarValues::*;
		private import SI::*;
		private import VectorFunctions::*;
		private import StateSpaceRepresentation::*;
		private import StateSpaceIntegration::*;

		action dyn : ` + header + ` {
			in :>> input = VectorOf((0.0));
			` + body + `
		}
	}`
}

const decayCalcs = `
	calc :>> getDerivative {
		in input : Input;
		in stateSpace : StateSpace;
		return : StateDerivative = (0.0 - 0.5) * stateSpace / 1 [s];
	}
	calc :>> getOutput {
		in input : Input;
		in stateSpace : StateSpace;
		return : Output = stateSpace;
	}
`

// runStateSpace runs action test::dyn of src to completion, returning the error
// its start or its run reports.
func runStateSpace(t *testing.T, src string) error {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "dyn", ast.DefAction)
	if sym == nil {
		t.Fatal("action dyn not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		return err
	}
	return exec.RunToCompletion()
}

func expectStateSpaceError(t *testing.T, err, want error, says ...string) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	for _, s := range says {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error %q does not say %q", err, s)
		}
	}
}

func testStateSpaceStateNotAVector(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = 1.0;
		:>> timeStep = 0.1 [s];
		:>> stopTime = 1 [s];
	`+decayCalcs))
	expectStateSpaceError(t, err, ErrStateSpaceValue, "stateSpace of action dyn", "not a vector")
}

func testStateSpaceReturnParameter(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		return result : StateSpace;
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = 0.1 [s];
		:>> stopTime = 1 [s];
	`+decayCalcs))
	expectStateSpaceError(t, err, ErrActionResultParameter, "action dyn declares `return result`", "write `out result`")
}

func testStateSpaceStateWithoutAnInitialValue(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> timeStep = 0.1 [s];
		:>> stopTime = 1 [s];
	`+decayCalcs))
	expectStateSpaceError(t, err, ErrStateSpaceValue, "action dyn binds no stateSpace")
}

func testStateSpaceDerivativeLeftAbstract(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = 0.1 [s];
		:>> stopTime = 1 [s];
		calc :>> getOutput {
			in input : Input;
			in stateSpace : StateSpace;
			return : Output = stateSpace;
		}
	`))
	expectStateSpaceError(t, err, lower.ErrUnsupportedStateSpace, "action dyn leaves getDerivative abstract")
}

func testStateSpaceDifferenceLeftAbstract(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("DiscreteStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = 1 [s];
		:>> stopTime = 3 [s];
		calc :>> getOutput {
			in input : Input;
			in stateSpace : StateSpace;
			return : Output = stateSpace;
		}
	`))
	expectStateSpaceError(t, err, lower.ErrUnsupportedStateSpace, "action dyn leaves getDifference abstract")
}

func testStateSpaceDerivativeNotAVector(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = 0.1 [s];
		:>> stopTime = 1 [s];
		calc :>> getDerivative {
			in input : Input;
			in stateSpace : StateSpace;
			return : Real = 0.0 - 0.5;
		}
		calc :>> getOutput {
			in input : Input;
			in stateSpace : StateSpace;
			return : Output = stateSpace;
		}
	`))
	expectStateSpaceError(t, err, ErrStateSpaceValue, "getDerivative of action dyn", "not a vector")
}

func testStateSpaceOutputNotAVector(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = 0.1 [s];
		:>> stopTime = 1 [s];
		calc :>> getDerivative {
			in input : Input;
			in stateSpace : StateSpace;
			return : StateDerivative = (0.0 - 0.5) * stateSpace / 1 [s];
		}
		calc :>> getOutput {
			in input : Input;
			in stateSpace : StateSpace;
			return : Real = stateSpace.elements#(1);
		}
	`))
	expectStateSpaceError(t, err, ErrStateSpaceValue, "output of action dyn", "not a vector")
}

func testStateSpaceStepNotStated(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics", `
		:>> stateSpace = VectorOf((1.0));
	`+decayCalcs))
	expectStateSpaceError(t, err, ErrStateSpaceStep, "action dyn states no timeStep", lower.FixedStepDynamicsFQN)
}

func testStateSpaceStepZero(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = 0 [s];
		:>> stopTime = 1 [s];
	`+decayCalcs))
	expectStateSpaceError(t, err, ErrStateSpaceStep, "timeStep of action dyn", "not a positive duration")
}

func testStateSpaceStepNegative(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = -0.1 [s];
		:>> stopTime = 1 [s];
	`+decayCalcs))
	expectStateSpaceError(t, err, ErrStateSpaceStep, "timeStep of action dyn", "not a positive duration")
}

func testStateSpaceUnknownIntegrator(t *testing.T) {
	err := runStateSpace(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		private import VectorFunctions::*;
		private import StateSpaceRepresentation::*;
		private import StateSpaceIntegration::*;

		calc def Heun :> Integrate;

		action dyn : ContinuousStateSpaceDynamics, FixedStepDynamics {
			in :>> input = VectorOf((0.0));
			:>> stateSpace = VectorOf((1.0));
			:>> timeStep = 0.1 [s];
			:>> stopTime = 1 [s];
			`+decayCalcs+`
			calc :>> getNextState {
				calc :>> integrate : Heun;
			}
		}
	}`)
	expectStateSpaceError(t, err, lower.ErrUnsupportedStateSpace, "binds integrate to Heun", "Euler", "RK4")
}

// A derivative growing faster than the state overflows within a few steps; the
// step that leaves a component non-finite is refused with the instant it happened at.
func testStateSpaceDivergentState(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((10.0));
		:>> timeStep = 1 [s];
		:>> stopTime = 100 [s];
		calc :>> getDerivative {
			in input : Input;
			in stateSpace : StateSpace;
			return : StateDerivative = stateSpace * stateSpace.elements#(1) * stateSpace.elements#(1) / 1 [s];
		}
		calc :>> getOutput {
			in input : Input;
			in stateSpace : StateSpace;
			return : Output = stateSpace;
		}
	`))
	expectStateSpaceError(t, err, ErrStateSpaceDiverged, "of action dyn at t=", "not a finite Real")
}

// A step so long that the next instant is not a finite number is refused before
// the token is parked, so the clock never advances to infinity; the state itself
// stays finite here, the derivative being zero.
func testStateSpaceStepPastLastInstant(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = 1.0e308 [s];
		calc :>> getDerivative {
			in input : Input;
			in stateSpace : StateSpace;
			return : StateDerivative = 0.0 * stateSpace / 1 [s];
		}
		calc :>> getOutput {
			in input : Input;
			in stateSpace : StateSpace;
			return : Output = stateSpace;
		}
	`))
	expectStateSpaceError(t, err, ErrStateSpaceStep, "step 2 of action dyn", "last instant the clock can hold")
}

func testStateSpaceGuardNotANumber(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = 0.1 [s];
		:>> stopTime = 1 [s];
		event occurrence low : ZeroCrossing {
			:>> guard = stateSpace.elements#(1) < 0.5;
		}
	`+decayCalcs))
	expectStateSpaceError(t, err, ErrStateSpaceValue, "guard of zero crossing low of action dyn", "not a number")
}
