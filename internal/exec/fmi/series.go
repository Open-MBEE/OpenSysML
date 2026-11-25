package fmi

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// decimalRat is a float's shortest round-trip decimal as an exact rational: a
// 0.1 s step then stays 1/10, so a 0.3 s run keeps its final sample.
func decimalRat(f float64) *big.Rat {
	r, ok := new(big.Rat).SetString(strconv.FormatFloat(f, 'g', -1, 64))
	if !ok {
		return new(big.Rat).SetFloat64(f)
	}
	return r
}

// Reserved ToolVariable names for a trajectory: sample count or spacing,
// sample times and the `fmi:series:<name>` output prefix.
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

// seriesPlan is what one call asks the runner for beside its scalar outputs:
// the number of samples and the FMU variables to sample, in call order.
type seriesPlan struct {
	samples  int
	vars     []string
	times    bool
	instants []float64
}

// requested reports whether any output of the call wants a series.
func (p seriesPlan) requested() bool { return p.times || len(p.vars) > 0 }

// planSeries collects the call's series outputs and resolves their sample
// count and requested instants before the runner starts.
func planSeries(call *runtime.ToolCall, d *Description, startTime, stopTime float64, samples, interval *float64) (seriesPlan, error) {
	var plan seriesPlan
	seen := make(map[string]bool)
	for _, out := range call.Outputs {
		switch kind, _, name := classifyOutput(out.Variable, d); kind {
		case sampleTimes:
			plan.times = true
		case sampleSeries:
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
	if interval != nil {
		plan.instants = make([]float64, count)
		start, step := decimalRat(startTime), decimalRat(*interval)
		for i := range plan.instants {
			r := new(big.Rat).Add(start, new(big.Rat).Mul(step, new(big.Rat).SetInt64(int64(i))))
			plan.instants[i], _ = r.Float64()
		}
	}
	return plan, nil
}

// sampleCount resolves the sample count from a count or interval over the
// experiment. Both names together are refused; the count is bounded.
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
	if interval == nil {
		return defaultSeriesSamples, nil
	}
	if *interval <= 0 || math.IsNaN(*interval) || math.IsInf(*interval, 0) {
		return 0, fault(fmt.Sprintf("%s must be a positive finite number of seconds, got %v", seriesIntervalInput, *interval))
	}
	span := new(big.Rat).Sub(decimalRat(stopTime), decimalRat(startTime))
	if span.Sign() <= 0 {
		return 0, fault(fmt.Sprintf("%s needs stopTime after startTime", seriesIntervalInput))
	}
	step := decimalRat(*interval)
	if step.Sign() <= 0 {
		return 0, fault(fmt.Sprintf("%s must be a positive finite number of seconds, got %v", seriesIntervalInput, *interval))
	}
	quotient := new(big.Rat).Quo(span, step)
	floor := new(big.Int).Quo(quotient.Num(), quotient.Denom())
	count := new(big.Int).Add(floor, big.NewInt(1))
	if count.Cmp(big.NewInt(2)) < 0 {
		return 0, fault(fmt.Sprintf("%s = %v samples no more than one point over the experiment", seriesIntervalInput, *interval))
	}
	if count.Cmp(big.NewInt(maxSeriesSamples)) > 0 {
		return 0, fault(fmt.Sprintf("%s asks for %s samples, more than the limit of %d", seriesIntervalInput, count.String(), maxSeriesSamples))
	}
	return int(count.Int64()), nil
}

// checkSeriesVariables refuses a trajectory naming an unknown, input or
// array variable of the FMU.
func checkSeriesVariables(call *runtime.ToolCall, d *Description) error {
	for _, out := range call.Outputs {
		kind, _, name := classifyOutput(out.Variable, d)
		if kind != sampleSeries {
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
