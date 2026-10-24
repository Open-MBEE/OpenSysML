package runtime

import "testing"

// TestRuntimeRobustnessStateSpace holds the failure modes of a state-space run added
// since TestStateSpaceRobustness: each is a typed error, never a panic or a hang.
func TestRuntimeRobustnessStateSpace(t *testing.T) {
	t.Run("stop_that_is_negative", testStateSpaceStopNegative)
}

func testStateSpaceStopNegative(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = 0.1 [s];
		:>> stopTime = -1 [s];
	`+decayCalcs))
	expectStateSpaceError(t, err, errStateSpaceStep, "stopTime of action dyn", "not a duration")
}
