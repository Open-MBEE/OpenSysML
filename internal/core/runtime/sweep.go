package runtime

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A sweep is tool-defined orchestration: it makes one ordinary analysis or calc
// run per row with one parameter bound to that row's value, and reports what
// each run produced. The language states nothing about it.

// Typed refusals a sweep reports rather than running anything.
var (
	// ErrSweepRange reports a range no sequence of values follows from: a
	// non-numeric endpoint, endpoints measuring different things, a zero step, a
	// step whose sign never reaches the endpoint, or a real range stating none.
	ErrSweepRange = errors.New("invalid sweep range")
	// ErrSweepParameter reports a parameter the target declares none of, or one
	// the invocation already binds.
	ErrSweepParameter = errors.New("invalid sweep parameter")
	// ErrSweepSamples reports a sample request that draws nothing, states a step,
	// or asks for a distribution no library in this build states.
	ErrSweepSamples = errors.New("invalid sample request")
	// ErrSweepBudget reports a sweep asking for more runs than its budget allows.
	ErrSweepBudget = errors.New("sweep run budget exceeded")
	// ErrSweepEmpty reports a sweep or sample naming no range at all.
	ErrSweepEmpty = errors.New("no sweep range")
	// ErrSweepDistribution reports a distribution asked for by name. Sampling is
	// uniform over a range: the bundled library states no probability
	// distributions, so nothing in the model gives a named one a meaning.
	ErrSweepDistribution = errors.New("no distribution library")
)

// SweepRange is one parameter's range as values: the endpoints it runs between
// and the step it advances by, which a sampled range leaves unstated.
type SweepRange struct {
	Param   string
	From    Value
	To      Value
	Step    Value
	HasStep bool
}

// SweepBinding is one parameter bound to the value one row runs with.
type SweepBinding struct {
	Param string
	Value Value
}

// SweepPlan is what a sweep asks for: the ranges its parameters take, and for a
// sampled sweep the number of draws and the seed they are drawn from.
type SweepPlan struct {
	Ranges  []SweepRange
	Sampled bool
	Samples int64
	Seed    uint64
}

// SweepRunResult is what one run of a sweep produced. A calc's returned value is
// reported as an output named "result", so a calc row and a case row read alike.
type SweepRunResult struct {
	Outputs  []CalcOutputValue
	Verdicts []AnalysisVerdict
}

// SweepRun makes one run of a sweep with the parameters bound as the row states.
// An error is that row's failure, not the table's.
type SweepRun func(bindings []SweepBinding) (SweepRunResult, error)

// SweepRow is one run of a sweep: what it was given, what it produced, how long
// it took, and what stopped it when it failed.
type SweepRow struct {
	Bindings []SweepBinding
	Outputs  []CalcOutputValue
	Verdicts []AnalysisVerdict
	Elapsed  time.Duration
	Err      error
}

// SweepTable is every run of one sweep, in the order they were made: a swept
// table runs lexicographically over its parameters in the order they were
// given, each from its first endpoint; a sampled table runs in draw order.
type SweepTable struct {
	Target  string
	Params  []string
	Sampled bool
	Seed    uint64
	Rows    []SweepRow
}

// sampleStream separates the generator's two seed words, so one seed still
// selects one whole PCG state.
const sampleStream uint64 = 0x9E3779B97F4A7C15

// NewSampleSource is the generator a sampled sweep draws from: math/rand/v2's
// PCG seeded from the given seed alone, so one seed draws one sequence of
// values on every platform and every build.
func NewSampleSource(seed uint64) *rand.Rand {
	// #nosec G404 -- a reproducible table needs a stated generator, not a cryptographic one.
	return rand.New(rand.NewPCG(seed, seed^sampleStream))
}

// RunSweep makes one run per row of the plan and reports the table. A run that
// failed is that row's typed error; the table is completed either way. The plan
// itself is refused before any run is made.
func (ctx *Context) RunSweep(target string, plan SweepPlan, run SweepRun) (SweepTable, error) {
	rows, err := ctx.sweepBindings(plan)
	if err != nil {
		return SweepTable{}, err
	}
	table := SweepTable{
		Target:  target,
		Params:  make([]string, 0, len(plan.Ranges)),
		Sampled: plan.Sampled,
		Seed:    plan.Seed,
		Rows:    make([]SweepRow, 0, len(rows)),
	}
	for _, r := range plan.Ranges {
		table.Params = append(table.Params, r.Param)
	}
	for _, bindings := range rows {
		row := SweepRow{Bindings: bindings}
		started := time.Now()
		result, err := run(bindings)
		row.Elapsed = time.Since(started)
		if err != nil {
			row.Err = err
		} else {
			row.Outputs, row.Verdicts = result.Outputs, result.Verdicts
		}
		table.Rows = append(table.Rows, row)
	}
	return table, nil
}

// SweepRunBudget is the number of runs one sweep may ask for.
func (ctx *Context) SweepRunBudget() int64 { return ctx.maxSweepRuns }

// sweepBindings is every row of a plan, in the order it is run: the cartesian
// product of the swept ranges, or one row per draw of a sampled one.
func (ctx *Context) sweepBindings(plan SweepPlan) ([][]SweepBinding, error) {
	if len(plan.Ranges) == 0 {
		return nil, fmt.Errorf("%w: name a range as <parameter>=<from>..<to>", ErrSweepEmpty)
	}
	seen := make(map[string]bool, len(plan.Ranges))
	for _, r := range plan.Ranges {
		if r.Param == "" {
			return nil, fmt.Errorf("%w: a range names no parameter", ErrSweepParameter)
		}
		if seen[r.Param] {
			return nil, fmt.Errorf("%w: %s is swept twice", ErrSweepParameter, r.Param)
		}
		seen[r.Param] = true
	}
	if plan.Sampled {
		return ctx.sampledBindings(plan)
	}
	return ctx.sweptBindings(plan)
}

// sweptBindings enumerates each range and takes the cartesian product, the
// first parameter varying slowest so the rows read in the order given.
func (ctx *Context) sweptBindings(plan SweepPlan) ([][]SweepBinding, error) {
	columns := make([][]Value, len(plan.Ranges))
	total := int64(1)
	for i, r := range plan.Ranges {
		values, err := r.enumerate(ctx.maxSweepRuns)
		if err != nil {
			return nil, err
		}
		columns[i] = values
		if total > ctx.maxSweepRuns/int64(len(values)) {
			return nil, ctx.sweepBudgetError()
		}
		total *= int64(len(values))
	}
	rows := make([][]SweepBinding, 0, total)
	index := make([]int, len(columns))
	for {
		row := make([]SweepBinding, len(columns))
		for i := range columns {
			row[i] = SweepBinding{Param: plan.Ranges[i].Param, Value: columns[i][index[i]]}
		}
		rows = append(rows, row)
		pos := len(columns) - 1
		for pos >= 0 {
			index[pos]++
			if index[pos] < len(columns[pos]) {
				break
			}
			index[pos] = 0
			pos--
		}
		if pos < 0 {
			return rows, nil
		}
	}
}

// sampledBindings draws one value per parameter per row, the parameters in the
// order they were given, so the draws pair up into rows rather than multiplying.
func (ctx *Context) sampledBindings(plan SweepPlan) ([][]SweepBinding, error) {
	if plan.Samples <= 0 {
		return nil, fmt.Errorf("%w: draw at least one sample, got %d", ErrSweepSamples, plan.Samples)
	}
	if plan.Samples > ctx.maxSweepRuns {
		return nil, ctx.sweepBudgetError()
	}
	prepared := make([]sweepBounds, len(plan.Ranges))
	for i, r := range plan.Ranges {
		if r.HasStep {
			return nil, fmt.Errorf(
				"%w: sampled range %s states a step, which only a swept range advances by",
				ErrSweepSamples, r.Param,
			)
		}
		bounds, err := r.endpoints()
		if err != nil {
			return nil, err
		}
		prepared[i] = bounds
	}
	source := NewSampleSource(plan.Seed)
	rows := make([][]SweepBinding, 0, plan.Samples)
	for range plan.Samples {
		row := make([]SweepBinding, len(prepared))
		for i := range prepared {
			row[i] = SweepBinding{Param: plan.Ranges[i].Param, Value: prepared[i].draw(source)}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// sweepBudgetError is the refusal of a plan asking for more runs than allowed.
func (ctx *Context) sweepBudgetError() error {
	return fmt.Errorf("%w: at most %d run(s) per sweep (raise %s)",
		ErrSweepBudget, ctx.maxSweepRuns, MaxSweepRunsEnvVar)
}

// sweepBounds is a range as arithmetic reads it: magnitudes in the unit its
// first endpoint carries, and whether its values are Integers.
type sweepBounds struct {
	from, to float64
	step     float64
	isInt    bool
	unit     semantics.Unit
	quantity bool
}

// scalar is one endpoint's magnitude and unit.
type sweepScalar struct {
	num      semantics.Value
	unit     semantics.Unit
	quantity bool
}

// sweepScalarOf reads an endpoint: a bare number or a quantity, never anything
// a range cannot advance through.
func sweepScalarOf(v Value, what string) (sweepScalar, error) {
	switch v.Kind {
	case ValConst:
		if !v.Const.IsNumeric() {
			return sweepScalar{}, fmt.Errorf("%w: %s is %s, not a number",
				ErrSweepRange, what, semantics.FormatConst(v.Const))
		}
		return sweepScalar{num: v.Const}, nil
	case ValQuantity:
		q := v.Quantity()
		if q == nil || !q.Num.IsNumeric() {
			return sweepScalar{}, fmt.Errorf("%w: %s is not a numeric quantity", ErrSweepRange, what)
		}
		return sweepScalar{num: q.Num, unit: q.Unit, quantity: true}, nil
	}
	return sweepScalar{}, fmt.Errorf("%w: %s is a %s, not a number or a quantity",
		ErrSweepRange, what, v.Kind)
}

// magnitudeIn expresses the scalar in unit, which requires the two to measure
// the same thing: a bare number and a quantity never do.
func (s sweepScalar) magnitudeIn(unit semantics.Unit, quantity bool, what string) (float64, error) {
	if s.quantity != quantity {
		return 0, fmt.Errorf("%w: %s and the first endpoint do not both carry a unit", ErrSweepRange, what)
	}
	if !quantity {
		return s.num.AsReal(), nil
	}
	q := Quantity{Num: s.num, Unit: s.unit}
	m, err := q.ConvertTo(unit)
	if err != nil {
		return 0, fmt.Errorf("%w: %s is expressed in %s, not in %s", ErrSweepRange, what, s.unit, unit)
	}
	return m, nil
}

// endpoints reads a range's endpoints, expressed in the unit its first one
// carries. A range between Integers takes Integer values. A sampled range needs
// nothing more, since it draws over the endpoints rather than stepping.
func (r SweepRange) endpoints() (sweepBounds, error) {
	from, err := sweepScalarOf(r.From, "range start "+FormatValue(r.From))
	if err != nil {
		return sweepBounds{}, err
	}
	to, err := sweepScalarOf(r.To, "range end "+FormatValue(r.To))
	if err != nil {
		return sweepBounds{}, err
	}
	bounds := sweepBounds{unit: from.unit, quantity: from.quantity}
	bounds.from = from.num.AsReal()
	if bounds.to, err = to.magnitudeIn(from.unit, from.quantity, "range end "+FormatValue(r.To)); err != nil {
		return sweepBounds{}, err
	}
	bounds.isInt = whole(from.num, bounds.from) && whole(to.num, bounds.to)
	return bounds, nil
}

// bounds reads a range a sweep steps through: its endpoints and its step. A
// range between Integers steps by one where it states none; a real range with
// no step is refused, since no step is the obvious one between two reals.
func (r SweepRange) bounds() (sweepBounds, error) {
	bounds, err := r.endpoints()
	if err != nil {
		return sweepBounds{}, err
	}
	if !r.HasStep {
		if !bounds.isInt {
			return sweepBounds{}, fmt.Errorf(
				"%w: %s=%s..%s states no step; only a range between Integers steps by one, a real range needs `:<step>`",
				ErrSweepRange, r.Param, FormatValue(r.From), FormatValue(r.To),
			)
		}
		bounds.step = 1
		if bounds.to < bounds.from {
			bounds.step = -1
		}
		return bounds, nil
	}
	step, err := sweepScalarOf(r.Step, "step "+FormatValue(r.Step))
	if err != nil {
		return sweepBounds{}, err
	}
	if bounds.step, err = step.magnitudeIn(bounds.unit, bounds.quantity, "step "+FormatValue(r.Step)); err != nil {
		return sweepBounds{}, err
	}
	bounds.isInt = bounds.isInt && whole(step.num, bounds.step)
	if bounds.step == 0 {
		return sweepBounds{}, fmt.Errorf("%w: %s steps by zero, which never reaches %s",
			ErrSweepRange, r.Param, FormatValue(r.To))
	}
	if bounds.to != bounds.from && (bounds.to-bounds.from)*bounds.step < 0 {
		return sweepBounds{}, fmt.Errorf("%w: %s steps by %s away from %s",
			ErrSweepRange, r.Param, FormatValue(r.Step), FormatValue(r.To))
	}
	return bounds, nil
}

// whole reports whether a magnitude is an Integer as written and stayed one
// after being expressed in the range's unit.
func whole(num semantics.Value, magnitude float64) bool {
	return num.Kind == semantics.ValInt && magnitude == math.Trunc(magnitude)
}

// enumerate is every value of a swept range, from its start towards its end,
// including the end where a step lands on it. Values are computed from the
// start rather than accumulated, so a real step does not drift.
func (r SweepRange) enumerate(limit int64) ([]Value, error) {
	bounds, err := r.bounds()
	if err != nil {
		return nil, err
	}
	span := (bounds.to - bounds.from) / bounds.step
	// A step landing within a rounding error of the endpoint reaches it: the
	// notation says 0.0..1.0:0.1 has eleven values, and binary reals do not.
	count := int64(math.Floor(span+stepTolerance(span))) + 1
	if count < 1 {
		count = 1
	}
	if count > limit {
		return nil, fmt.Errorf("%w: %s=%s..%s takes %d run(s), at most %d allowed (raise %s)",
			ErrSweepBudget, r.Param, FormatValue(r.From), FormatValue(r.To), count, limit, MaxSweepRunsEnvVar)
	}
	values := make([]Value, 0, count)
	for i := int64(0); i < count; i++ {
		values = append(values, bounds.value(bounds.from+float64(i)*bounds.step))
	}
	return values, nil
}

// stepTolerance is the rounding error a count of steps is allowed, scaled to
// how many steps were counted.
func stepTolerance(span float64) float64 {
	return 1e-9 * math.Max(1, math.Abs(span))
}

// draw is one uniform value of a sampled range: an Integer range draws over its
// endpoints inclusively, a real one over [from, to).
func (b sweepBounds) draw(source *rand.Rand) Value {
	lo, hi := b.from, b.to
	if hi < lo {
		lo, hi = hi, lo
	}
	if b.isInt {
		return b.value(float64(int64(lo) + source.Int64N(int64(hi)-int64(lo)+1)))
	}
	return b.value(lo + source.Float64()*(hi-lo))
}

// value is a magnitude as the range's own kind of value: an Integer or a real,
// carrying the unit its first endpoint was expressed in.
func (b sweepBounds) value(magnitude float64) Value {
	num := semantics.Value{Kind: semantics.ValReal, Real: magnitude}
	if b.isInt {
		num = semantics.Value{Kind: semantics.ValInt, Int: int64(math.Round(magnitude))}
	}
	if !b.quantity {
		return Value{Kind: ValConst, Const: num}
	}
	return NewQuantityValue(&Quantity{Num: num, Unit: b.unit.Clone()})
}

// InputParameterNames reports the input parameters a calc or analysis case
// declares, in the order an invocation's positional arguments bind them.
func (ctx *Context) InputParameterNames(sym *symbols.Symbol) ([]string, error) {
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(shape.Params))
	for i := range shape.Params {
		names = append(names, shape.Params[i].Name)
	}
	return names, nil
}

// CheckSweepParameters refuses a plan whose parameter the target does not
// declare, or which the invocation already binds — by name, or by holding the
// position that parameter is bound from.
func (ctx *Context) CheckSweepParameters(sym *symbols.Symbol, plan SweepPlan, positional int, named []string) error {
	declared, err := ctx.InputParameterNames(sym)
	if err != nil {
		return err
	}
	bound := make(map[string]bool, len(named)+positional)
	for _, name := range named {
		bound[name] = true
	}
	for i := 0; i < positional && i < len(declared); i++ {
		bound[declared[i]] = true
	}
	for _, r := range plan.Ranges {
		if !contains(declared, r.Param) {
			return fmt.Errorf("%w: %s declares no input parameter %q (it declares %v)",
				ErrSweepParameter, ctx.qualifiedSymbolName(sym), r.Param, declared)
		}
		if bound[r.Param] {
			return fmt.Errorf("%w: %s is both an argument of the invocation and swept",
				ErrSweepParameter, r.Param)
		}
	}
	return nil
}
