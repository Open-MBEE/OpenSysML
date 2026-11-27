package runtime

import "testing"

// The executor reads its inputs after SetInputs returns, so it keeps its own
// copy: a caller reusing the map does not change a run already set up.
func TestSetInputsKeepsItsOwnCopy(t *testing.T) {
	inputs := map[string]Value{"amount": realOf(1)}
	e := &ActionExecutor{}
	e.SetInputs(inputs)
	inputs["amount"] = realOf(2)
	inputs["other"] = realOf(3)
	if got := e.inputs["amount"]; got != realOf(1) || len(e.inputs) != 1 {
		t.Fatalf("the executor's inputs followed the caller's map: %v", e.inputs)
	}
}
