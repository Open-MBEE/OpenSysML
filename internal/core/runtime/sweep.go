package runtime

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/bits"
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
	// The object the run was about, where a case ran on one.
	Subject *Instance
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
	// The object this run's verdicts are about, where a case ran on one.
	Subject *Instance
	Elapsed time.Duration
	Err     error
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
// itself is refused before any run is made, and a caller that goes away between
// runs takes the rest of the table with it.
func (ctx *Context) RunSweep(stop context.Context, target string, plan SweepPlan, run SweepRun) (SweepTable, error) {
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
		if err := stop.Err(); err != nil {
			return SweepTable{}, err
		}
		row := SweepRow{Bindings: bindings}
		started := time.Now()
		result, err := run(bindings)
		row.Elapsed = time.Since(started)
		if err != nil {
			row.Err = err
		} else {
			row.Outputs, row.Verdicts = result.Outputs, result.Verdicts
			row.Subject = result.Subject
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
// first endpoint carries, and whether its values are Integers. A range between
// Integers also keeps them exactly, which float64 stops doing beyond 2^53.
type sweepBounds struct {
	from, to float64
	step     float64
	isInt    bool
	intFrom  int64
	intTo    int64
	intStep  int64
	unit     semantics.Unit
	quantity bool
}

// exactFloatInt is the largest magnitude float64 counts by ones through.
const exactFloatInt = 1 << 53

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
	fromInt, fromWhole := from.exactInt(bounds.unit, bounds.from)
	toInt, toWhole := to.exactInt(bounds.unit, bounds.to)
	bounds.isInt = fromWhole && toWhole
	bounds.intFrom, bounds.intTo = fromInt, toInt
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
		bounds.step, bounds.intStep = 1, 1
		if bounds.intTo < bounds.intFrom {
			bounds.step, bounds.intStep = -1, -1
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
	stepInt, stepWhole := step.exactInt(bounds.unit, bounds.step)
	bounds.isInt = bounds.isInt && stepWhole
	bounds.intStep = stepInt
	if bounds.step == 0 {
		return sweepBounds{}, fmt.Errorf("%w: %s steps by zero, which never reaches %s",
			ErrSweepRange, r.Param, FormatValue(r.To))
	}
	if bounds.stepsAway() {
		return sweepBounds{}, fmt.Errorf("%w: %s steps by %s away from %s",
			ErrSweepRange, r.Param, FormatValue(r.Step), FormatValue(r.To))
	}
	return bounds, nil
}

// stepsAway reports whether the step leads away from the range's end.
func (b sweepBounds) stepsAway() bool {
	if b.isInt {
		return b.intTo != b.intFrom && (b.intTo > b.intFrom) != (b.intStep > 0)
	}
	return b.to != b.from && (b.to-b.from)*b.step < 0
}

// exactInt is the scalar as the Integer it was written as, expressed in the
// range's unit, where the range still takes Integer values.
func (s sweepScalar) exactInt(unit semantics.Unit, magnitude float64) (int64, bool) {
	if s.num.Kind != semantics.ValInt {
		return 0, false
	}
	if !s.quantity {
		return s.num.Int, true
	}
	if mul, div, ok := scaleRatio(s.unit.Term.Scale, unit.Term.Scale); ok {
		return exactScaled(s.num.Int, mul, div)
	}
	if magnitude != math.Trunc(magnitude) || math.Abs(magnitude) > exactFloatInt {
		return 0, false
	}
	return int64(magnitude), true
}

// exactScaleFactor bounds a scale factor's parts, so that the ratio between two
// of them is a product float64 holds exactly.
const exactScaleFactor = 1 << 26

// scaleRatio expresses a magnitude given over from in to, as the whole ratio
// mul/div where both scale factors are whole numbers of that size.
func scaleRatio(from, to semantics.Scale) (mul, div int64, ok bool) {
	for _, part := range [...]float64{from.Num, from.Den, to.Num, to.Den} {
		if part != math.Trunc(part) || part <= 0 || part > exactScaleFactor {
			return 0, 0, false
		}
	}
	return int64(from.Num * to.Den), int64(from.Den * to.Num), true
}

// exactScaled is n scaled by mul/div, where that leaves an Integer exactly:
// the product must fit and the division must come out even.
func exactScaled(n, mul, div int64) (int64, bool) {
	magnitude := unsignedInt(n)
	if n < 0 {
		magnitude = -magnitude
	}
	hi, lo := bits.Mul64(magnitude, unsignedInt(mul))
	if hi != 0 || lo%unsignedInt(div) != 0 {
		return 0, false
	}
	scaled := lo / unsignedInt(div)
	if n < 0 {
		if scaled > 1<<63 {
			return 0, false
		}
		return signedInt(-scaled), true
	}
	if scaled > math.MaxInt64 {
		return 0, false
	}
	return signedInt(scaled), true
}

// enumerate is every value of a swept range, from its start towards its end,
// including the end where a step lands on it. Values are computed from the
// start rather than accumulated, so a real step does not drift.
func (r SweepRange) enumerate(limit int64) ([]Value, error) {
	bounds, err := r.bounds()
	if err != nil {
		return nil, err
	}
	count := bounds.count()
	if count > unsignedInt(limit) {
		return nil, fmt.Errorf("%w: %s=%s..%s takes %d run(s), at most %d allowed (raise %s)",
			ErrSweepBudget, r.Param, FormatValue(r.From), FormatValue(r.To), count, limit, MaxSweepRunsEnvVar)
	}
	values := make([]Value, 0, count)
	for i := int64(0); i < signedInt(count); i++ {
		values = append(values, bounds.at(i))
	}
	return values, nil
}

// count is how many values the range takes, its end included where a step
// lands on it. An Integer range counts exactly; a real one counts within a
// rounding error, since 0.0..1.0:0.1 has eleven values and binary reals do not.
func (b sweepBounds) count() uint64 {
	if b.isInt {
		span := unsignedInt(b.intTo) - unsignedInt(b.intFrom)
		if b.intStep < 0 {
			span = unsignedInt(b.intFrom) - unsignedInt(b.intTo)
		}
		if span == math.MaxUint64 && b.intStepMagnitude() == 1 {
			return math.MaxUint64
		}
		return span/b.intStepMagnitude() + 1
	}
	span := (b.to - b.from) / b.step
	count := math.Floor(span+stepTolerance(span)) + 1
	if !(count > 1) {
		return 1
	}
	if count > math.MaxInt64 {
		return math.MaxInt64
	}
	return uint64(count)
}

// at is the range's value the given number of steps from its start. An Integer
// range steps in unsigned arithmetic, which reaches its endpoints exactly.
func (b sweepBounds) at(i int64) Value {
	if b.isInt {
		offset := unsignedInt(i) * b.intStepMagnitude()
		if b.intStep < 0 {
			return b.valueInt(signedInt(unsignedInt(b.intFrom) - offset))
		}
		return b.valueInt(signedInt(unsignedInt(b.intFrom) + offset))
	}
	return b.value(b.from + float64(i)*b.step)
}

// intStepMagnitude is how far one Integer step reaches, which the widest step
// only states unsigned.
func (b sweepBounds) intStepMagnitude() uint64 {
	if b.intStep < 0 {
		return -unsignedInt(b.intStep)
	}
	return unsignedInt(b.intStep)
}

// unsignedInt and signedInt are the two views of one Integer. A sweep counts and
// steps unsigned, so a range as wide as Integer arithmetic stays exact.
func unsignedInt(n int64) uint64 {
	// #nosec G115 -- the two's-complement image is the value meant, not an overflow.
	return uint64(n)
}

func signedInt(n uint64) int64 {
	// #nosec G115 -- the two's-complement image is the value meant, not an overflow.
	return int64(n)
}

// stepTolerance is the rounding error a count of steps is allowed, scaled to
// how many steps were counted.
func stepTolerance(span float64) float64 {
	return 1e-9 * math.Max(1, math.Abs(span))
}

// draw is one uniform value of a sampled range: an Integer range draws over its
// endpoints inclusively, a real one over [from, to).
func (b sweepBounds) draw(source *rand.Rand) Value {
	if b.isInt {
		lo, hi := b.intFrom, b.intTo
		if hi < lo {
			lo, hi = hi, lo
		}
		return b.valueInt(signedInt(unsignedInt(lo) + drawOffset(source, unsignedInt(hi)-unsignedInt(lo))))
	}
	lo, hi := b.from, b.to
	if hi < lo {
		lo, hi = hi, lo
	}
	return b.value(lo + source.Float64()*(hi-lo))
}

// drawOffset is a uniform offset from zero to width inclusive, drawn as an
// Int64N while the width leaves room for it so a wider range costs nothing.
func drawOffset(source *rand.Rand, width uint64) uint64 {
	switch {
	case width < math.MaxInt64:
		return unsignedInt(source.Int64N(signedInt(width) + 1))
	case width == math.MaxUint64:
		return source.Uint64()
	default:
		return source.Uint64N(width + 1)
	}
}

// value is a magnitude as the range's own kind of value: an Integer or a real,
// carrying the unit its first endpoint was expressed in.
func (b sweepBounds) value(magnitude float64) Value {
	num := semantics.Value{Kind: semantics.ValReal, Real: magnitude}
	if b.isInt {
		num = semantics.Value{Kind: semantics.ValInt, Int: int64(math.Round(magnitude))}
	}
	return b.carry(num)
}

// valueInt is an Integer of the range, carrying its unit.
func (b sweepBounds) valueInt(n int64) Value {
	return b.carry(semantics.Value{Kind: semantics.ValInt, Int: n})
}

// carry is the number in the unit the range's first endpoint was expressed in.
func (b sweepBounds) carry(num semantics.Value) Value {
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
	return shape.parameterNames(), nil
}

// CheckSweepParameters refuses a plan whose parameter the target does not
// declare, whose parameter is a case's subject, or which the invocation already
// binds — by name, or by holding the position that parameter is bound from.
func (ctx *Context) CheckSweepParameters(sym *symbols.Symbol, plan SweepPlan, positional int, named []string) error {
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return err
	}
	declared := shape.parameterNames()
	bound := make(map[string]bool, len(named)+positional)
	for _, name := range named {
		bound[name] = true
	}
	for i, name := range shape.positionalOrder(IsAnalysisSymbol(sym), bound) {
		if i >= positional {
			break
		}
		bound[name] = true
	}
	for _, r := range plan.Ranges {
		if !contains(declared, r.Param) {
			return fmt.Errorf("%w: %s declares no input parameter %q (it declares %v)",
				ErrSweepParameter, ctx.qualifiedSymbolName(sym), r.Param, declared)
		}
		if subject, ok := shape.subjectParameter(); ok && subject.Name == r.Param {
			return fmt.Errorf("%w: %s is the subject of %s, which an object binds, not a range",
				ErrSweepParameter, r.Param, ctx.qualifiedSymbolName(sym))
		}
		if bound[r.Param] {
			return fmt.Errorf("%w: %s is both an argument of the invocation and swept",
				ErrSweepParameter, r.Param)
		}
	}
	return nil
}

// positionalOrder is the parameters the invocation's positional arguments bind,
// in order: a case skips its subject and the parameters bound by name, as an
// analysis run binds them, while a calc binds every parameter by position.
func (shape *calcShape) positionalOrder(analysis bool, named map[string]bool) []string {
	names := make([]string, 0, len(shape.Params))
	for i := range shape.Params {
		param := &shape.Params[i]
		if analysis && (param.IsSubject || named[param.Name]) {
			continue
		}
		names = append(names, param.Name)
	}
	return names
}

// parameterNames is the parameters the target declares, in declaration order.
func (shape *calcShape) parameterNames() []string {
	names := make([]string, 0, len(shape.Params))
	for i := range shape.Params {
		names = append(names, shape.Params[i].Name)
	}
	return names
}
