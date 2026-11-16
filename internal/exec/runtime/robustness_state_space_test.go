package runtime

import "testing"

// TestRuntimeRobustnessStateSpace holds the failure modes of a state-space run added
// since TestStateSpaceRobustness: each is a typed error, never a panic or a hang.
func TestRuntimeRobustnessStateSpace(t *testing.T) {
	t.Run("stop_that_is_negative", testStateSpaceStopNegative)
	t.Run("two_flows_into_one_input", testStateSpaceTwoFlowsIntoOneInput)
	t.Run("flow_source_that_is_not_a_vector", testStateSpaceFlowSourceNotAVector)
	t.Run("flow_source_that_is_no_object", testStateSpaceFlowSourceNoObject)
}

func testStateSpaceStopNegative(t *testing.T) {
	err := runStateSpace(t, stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics", `
		:>> stateSpace = VectorOf((1.0));
		:>> timeStep = 0.1 [s];
		:>> stopTime = -1 [s];
	`+decayCalcs))
	expectStateSpaceError(t, err, errStateSpaceStep, "stopTime of action dyn", "not a duration")
}

func testStateSpaceTwoFlowsIntoOneInput(t *testing.T) {
	_, err := runCoupled(t, coupledDynamics(`
		flow source.ramp.output to sink.sum.input;
		flow source.ramp.stateSpace to sink.sum.input;`, ""))
	expectStateSpaceError(t, err, errStateSpaceValue, "input of action sum is the target of 2 flows")
}

func testStateSpaceFlowSourceNotAVector(t *testing.T) {
	_, err := runCoupled(t, coupledDynamics("flow source.label to sink.sum.input;", ""))
	expectStateSpaceError(t, err, errStateSpaceValue, "source.label, flowing into input of action sum", "not a vector")
}

func testStateSpaceFlowSourceNoObject(t *testing.T) {
	_, err := runCoupled(t, coupledDynamics("flow source.label.size to sink.sum.input;", ""))
	expectStateSpaceError(t, err, errStateSpaceValue, "read source.label.size, the source of the flow into it")
}
