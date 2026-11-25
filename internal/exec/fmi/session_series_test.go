package fmi

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// TestStepAndSeriesAreRefusedTogether locks the boundary between a session
// answering scalars per step and the one-shot runner answering a trajectory.
func TestStepAndSeriesAreRefusedTogether(t *testing.T) {
	description := &Description{CoSimulation: &Interface{}}
	step := runtime.ToolInput{Variable: StepVariable, Parameter: "t", Value: real(0.5)}
	series := runtime.ToolOutput{Variable: seriesOutputPrefix + "T", Parameter: "temps"}
	samples := runtime.ToolInput{Variable: seriesSamplesInput, Parameter: "n", Value: real(5)}
	for _, tc := range []struct {
		name    string
		inputs  []runtime.ToolInput
		outputs []runtime.ToolOutput
		refuse  bool
	}{
		{"step and series output", []runtime.ToolInput{step}, []runtime.ToolOutput{series}, true},
		{"step and series input", []runtime.ToolInput{step, samples}, nil, true},
		{"step alone", []runtime.ToolInput{step}, nil, false},
		{"series alone", []runtime.ToolInput{samples}, []runtime.ToolOutput{series}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := &runtime.ToolCall{ToolName: "fmi", Inputs: tc.inputs, Outputs: tc.outputs}
			_, plan, err := requestFields(call, description, "file:///probe.fmu")
			var refusal *SessionSeriesError
			if got := errors.As(err, &refusal); got != tc.refuse {
				t.Fatalf("error = %v, want SessionSeriesError = %v", err, tc.refuse)
			}
			if tc.refuse && !errors.Is(err, ErrSession) {
				t.Fatalf("error = %v, want it to unwrap to ErrSession", err)
			}
			if !tc.refuse && len(tc.outputs) > 0 && !plan.requested() {
				t.Fatal("a series output without a step must still carry a trajectory plan")
			}
		})
	}
}
