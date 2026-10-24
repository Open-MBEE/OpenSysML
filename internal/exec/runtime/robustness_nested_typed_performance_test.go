package runtime

import (
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessNestedTypedPerformance covers a nested action usage whose body
// binds features of the action it is typed by, where that action is performed by a tool
// or as state-space dynamics rather than by its flow: a shape the performance cannot
// run is refused as it is standalone, never completed over the bound initial values.
func TestRuntimeRobustnessNestedTypedPerformance(t *testing.T) {
	t.Run("dynamics_without_an_initial_state", testNestedDynamicsWithoutAnInitialState)
	t.Run("dynamics_with_an_integrator_the_runtime_does_not_provide", testNestedDynamicsUnknownIntegrator)
	t.Run("tool_nobody_registers", testNestedToolNobodyRegisters)
}

// nestedDynamicsSource nests a free-fall dynamics usage with the given body in an
// action `outer`; the usage's own members, not the definition's, state its run.
func nestedDynamicsSource(body string) string {
	return `package test {
		private import ScalarValues::*;
		private import SI::*;
		private import VectorFunctions::*;
		private import StateSpaceRepresentation::*;
		private import StateSpaceIntegration::*;

		attribute def FallState :> StateSpace;
		attribute def NoInput :> Input;
		attribute def HeightOutput :> Output;

		action def FreeFall :> ContinuousStateSpaceDynamics, FixedStepDynamics {
			in :>> input : NoInput = VectorOf((0.0));
			:>> stateSpace : FallState;
			calc :>> getDerivative {
				in input : NoInput;
				in stateSpace : FallState;
				return : StateDerivative = VectorOf((stateSpace.elements#(2), -9.81)) / 1 [s];
			}
			calc :>> getOutput {
				in input : NoInput;
				in stateSpace : FallState;
				return : HeightOutput = VectorOf((stateSpace.elements#(1)));
			}
		}

		calc def Heun :> Integrate;

		action outer {
			action fall : FreeFall {
				` + body + `
			}
		}
	}`
}

// runNestedPerformance executes the action `outer` of src and returns its error; a
// run that does not terminate fails the test.
func runNestedPerformance(t *testing.T, src string) error {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	done := make(chan error, 1)
	go func() {
		_, err := ctx.ExecuteAction(sym)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("outer completed, expected its nested performance to be refused")
		}
		return err
	case <-watchdog(10 * time.Second):
		t.Fatal("outer did not terminate")
		return nil
	}
}

func testNestedDynamicsWithoutAnInitialState(t *testing.T) {
	err := runNestedPerformance(t, nestedDynamicsSource(`
		:>> timeStep = 1 [s];
		:>> stopTime = 3 [s];
	`))
	expectStateSpaceError(t, err, errStateSpaceValue, "action fall binds no stateSpace")
}

func testNestedDynamicsUnknownIntegrator(t *testing.T) {
	err := runNestedPerformance(t, nestedDynamicsSource(`
		:>> stateSpace = VectorOf((100.0, 0.0));
		:>> timeStep = 1 [s];
		:>> stopTime = 3 [s];
		calc :>> getNextState { calc :>> integrate : Heun; }
	`))
	expectStateSpaceError(t, err, lower.ErrUnsupportedStateSpace, "action fall binds integrate to Heun")
}

func testNestedToolNobodyRegisters(t *testing.T) {
	err := runNestedPerformance(t, `package test {
		private import ScalarValues::*;
		private import AnalysisTooling::*;

		action def Solve {
			metadata ToolExecution { toolName = "nobody"; uri = "file:///solver"; }
			attribute mode : Integer default = 1;
			in x : Real;
			out y : Real;
		}

		action outer {
			action solve : Solve { :>> mode = 2; in :>> x = 3.0; }
		}
	}`)
	expectStateSpaceError(t, err, errToolNotRegistered, "tool 'nobody' is not registered")
}
