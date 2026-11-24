package fmi

import (
	"fmt"
	"math"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// Reserved ToolVariable names for a trajectory. A series output binds a sampled
// trajectory to a multi-valued parameter; `fmi:samples` fixes the number of
// samples and `fmi:sampleInterval` fixes their spacing in seconds. When neither
// is given the engine asks for defaultSeriesSamples evenly spaced samples.
const (
	seriesSamplesInput  = "fmi:samples"
	seriesIntervalInput = "fmi:sampleInterval"
	seriesTimeOutput    = "fmi:seriesTime"
	seriesOutputPrefix  = "fmi:series:"

	defaultSeriesSamples = 11
	maxSeriesSamples     = 10000
)

// seriesVariable reads the FMU variable a `fmi:series:<name>` output samples.
func seriesVariable(variable string) (string, bool) {
	return strings.CutPrefix(variable, seriesOutputPrefix)
}

// isSeriesOutput reports whether a ToolVariable names a series answer rather
// than a scalar one at stopTime.
func isSeriesOutput(variable string) bool {
	_, ok := seriesVariable(variable)
	return ok || variable == seriesTimeOutput
}

// seriesPlan is what one call asks the runner for beside its scalar outputs:
// the number of samples and the FMU variables to sample, in call order.
type seriesPlan struct {
	samples int
	vars    []string
	times   bool
}

// requested reports whether any output of the call wants a series.
func (p seriesPlan) requested() bool { return p.times || len(p.vars) > 0 }

// planSeries collects the series outputs of a call and resolves their sample
// count from `fmi:samples` or `fmi:sampleInterval` over the experiment's span.
// A call with no series output needs no plan and never sends a series request.
func planSeries(call *runtime.ToolCall, startTime, stopTime float64, samples, interval *float64) (seriesPlan, error) {
	var plan seriesPlan
	seen := make(map[string]bool)
	for _, out := range call.Outputs {
		if out.Variable == seriesTimeOutput {
			plan.times = true
			continue
		}
		if name, ok := seriesVariable(out.Variable); ok {
			if !seen[name] {
				seen[name] = true
				plan.vars = append(plan.vars, name)
			}
		}
	}
	if !plan.requested() {
		return plan, nil
	}
	count, err := sampleCount(startTime, stopTime, samples, interval)
	if err != nil {
		return seriesPlan{}, err
	}
	plan.samples = count
	return plan, nil
}

// sampleCount resolves the number of samples a trajectory carries. A sample
// count must be an integer of at least two; an interval must be positive and
// narrower than the experiment, so the trajectory holds more than one point.
// Naming both is a contradiction, refused rather than resolved by a silent
// precedence rule. Everything from the call is bounded before it reaches the
// runner.
func sampleCount(startTime, stopTime float64, samples, interval *float64) (int, error) {
	fault := func(detail string) error {
		return &runtime.ToolError{Tool: "fmi", Kind: runtime.ToolUnsentInput, Detail: detail}
	}
	if math.IsNaN(startTime) || math.IsInf(startTime, 0) || math.IsNaN(stopTime) || math.IsInf(stopTime, 0) {
		return 0, fault("a series needs finite start and stop times")
	}
	if samples != nil && interval != nil {
		return 0, fault(fmt.Sprintf("%s and %s were both sent; name one", seriesSamplesInput, seriesIntervalInput))
	}
	if samples != nil {
		n := *samples
		if n != math.Trunc(n) || n < 2 || n > maxSeriesSamples {
			return 0, fault(fmt.Sprintf("%s = %v is not an integer from 2 to %d", seriesSamplesInput, n, maxSeriesSamples))
		}
		return int(n), nil
	}
	span := stopTime - startTime
	if interval == nil {
		return defaultSeriesSamples, nil
	}
	if *interval <= 0 || math.IsNaN(*interval) || math.IsInf(*interval, 0) {
		return 0, fault(fmt.Sprintf("%s must be a positive finite number of seconds, got %v", seriesIntervalInput, *interval))
	}
	if span <= 0 {
		return 0, fault(fmt.Sprintf("%s needs stopTime after startTime", seriesIntervalInput))
	}
	n := int(math.Floor(span / *interval)) + 1
	if n < 2 {
		return 0, fault(fmt.Sprintf("%s = %v samples no more than one point over the experiment", seriesIntervalInput, *interval))
	}
	if n > maxSeriesSamples {
		return 0, fault(fmt.Sprintf("%s asks for %d samples, more than the limit of %d", seriesIntervalInput, n, maxSeriesSamples))
	}
	return n, nil
}

// checkSeriesVariables refuses a `fmi:series:<name>` output naming no variable
// of the FMU, one that cannot be read as an output, or an array: a trajectory
// samples a scalar at successive times and never collapses a series of rows.
func checkSeriesVariables(call *runtime.ToolCall, d *Description) error {
	for _, out := range call.Outputs {
		name, ok := seriesVariable(out.Variable)
		if !ok {
			continue
		}
		v, found := d.Variable(name)
		if !found {
			return &VariableError{FMU: call.URI, Variable: out.Variable,
				Detail: fmt.Sprintf("has no variable %q; its outputs are %s", name, candidates(d, false))}
		}
		switch v.Causality {
		case CausalityOutput, CausalityCalculatedParameter, CausalityLocal:
		default:
			return &VariableError{FMU: call.URI, Variable: out.Variable,
				Detail: fmt.Sprintf("variable %q is causality %s, not an output", name, v.Causality)}
		}
		if len(v.Dimensions) != 0 {
			return &VariableError{FMU: call.URI, Variable: out.Variable,
				Detail: fmt.Sprintf("variable %q is an array, which cannot be sampled as a trajectory", name)}
		}
	}
	return nil
}
