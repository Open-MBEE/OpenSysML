package fmi

import (
	"context"
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// TestOneShotRefusesStepAndSeriesTogether locks the typed refusal on the
// default host, which has no session tool runner installed.
func TestOneShotRefusesStepAndSeriesTogether(t *testing.T) {
	fmu := decayFMU(t)
	uri := fileURI(fmu)
	step := runtime.ToolInput{Variable: StepVariable, Parameter: "t", Value: real(0.5)}
	series := runtime.ToolOutput{Variable: seriesOutputPrefix + "y", Parameter: "ys"}
	for _, tc := range []struct {
		name    string
		outputs []runtime.ToolOutput
		series  bool
	}{
		{"step and series", []runtime.ToolOutput{series}, true},
		{"step alone", []runtime.ToolOutput{{Variable: "y", Parameter: "y"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := &runtime.ToolCall{ToolName: "fmi", URI: uri,
				Inputs: []runtime.ToolInput{step}, Outputs: tc.outputs}
			e := New(func() (string, error) { return "/no/such/runner", nil }).(*engine)
			q := analysis.Question{Kind: analysis.Compute, Compute: &analysis.ComputeAsk{Call: call}}

			check := func(err error) {
				t.Helper()
				if tc.series {
					var refusal *SessionSeriesError
					if !errors.As(err, &refusal) {
						t.Fatalf("error = %v, want *SessionSeriesError", err)
					}
					return
				}
				var refusal *SessionRequiredError
				if !errors.As(err, &refusal) {
					t.Fatalf("error = %v, want *SessionRequiredError", err)
				}
			}
			cov := e.Covers(nil, q)
			if cov.Covered {
				t.Fatal("Covers covered a stepped call without a session runner")
			}
			check(cov.Refusal)
			_, err := e.Run(context.Background(), nil, q, analysis.Budget{})
			check(err)
		})
	}
}
