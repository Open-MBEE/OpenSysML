package runtime

import (
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
	// step whose sign never reaches the endpoint, a real range stating none, or an
	// endpoint or step that is no value of the swept parameter's type.
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

// SweepRange is one parameter's range as values: its endpoints, the step a
// sampled range leaves unstated, and the type ResolveSweepPlan produces them in.
type SweepRange struct {
	Param   string
	From    Value
	To      Value
	Step    Value
	HasStep bool
	Type    SweepType
}

// SweepNumbers is which numbers a swept parameter's type takes, which decides
// the values its range produces and how a sampled range draws them.
type SweepNumbers uint8

const (
	// SweepAsWritten produces values as the endpoints are written — Integers
	// between Integers, reals otherwise — for a parameter that takes both or no type.
	SweepAsWritten SweepNumbers = iota
	// SweepIntegers produces Integers however the endpoints are written, and
	// refuses an endpoint or step that is not one.
	SweepIntegers
	// SweepReals produces reals however the endpoints are written.
	SweepReals
)

// SweepType is a swept parameter's declared type, which its range's values are
// produced in and each endpoint is admitted to as an argument is.
type SweepType struct {
	Numbers SweepNumbers
	// Untyped reports a parameter declaring no type, whose range is read as written.
	Untyped bool
	// Positive excludes zero, which no scalar type's numbers alone rule out.
	Positive bool
	decl     calcMemberDecl
	// num is the feature a quantity type's magnitude is bound to, nil for a scalar.
	num *symbols.Symbol
}

// Declared is the type the parameter declares, nil where it declares none or
// the range was never resolved.
func (t SweepType) Declared() *symbols.Symbol {
	if t.decl.Target == nil {
		return nil
	}
	return t.decl.Target.typ
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
	// Evaluations are the applications the run made of a calc held as a value,
	// in the order made.
	Evaluations []AnalysisEvaluation
}

// SweepRun makes one run of a sweep in ctx, the row's own context, with the
// parameters bound as the row states. An error is that row's failure, not the
// table's; what the run produced before failing may be returned beside it.
type SweepRun func(ctx *Context, bindings []SweepBinding) (SweepRunResult, error)

// SweepRow is one run of a sweep: what it was given, what it produced, how long
// it took, and what stopped it when it failed.
type SweepRow struct {
	Bindings []SweepBinding
	Outputs  []CalcOutputValue
	Verdicts []AnalysisVerdict
	// The object this run's verdicts are about, where a case ran on one.
	Subject     *Instance
	Evaluations []AnalysisEvaluation
	Elapsed     time.Duration
	Err         error
	// Context is the context the run was made in, which its outputs, subject and
	// evaluations are read through: no other context knows the objects they name.
	Context *Context
}

// SweepTable is every run of one sweep, in the order they were made: a swept
// table runs lexicographically over its parameters in the order they were
// given, each from its first endpoint; a sampled table runs in draw order.
type SweepTable struct {
	Target string
	Params []string
	// Types are the parameters' types as the plan resolved them, one per Param.
	Types   []SweepType
	Sampled bool
	Seed    uint64
	Rows    []SweepRow
}

// NewSweepTable is the table of a plan before any row is run: the target, the
// parameters and their types in plan order, and how the rows are drawn.
func NewSweepTable(target string, plan SweepPlan) SweepTable {
	table := SweepTable{
		Target:  target,
		Params:  make([]string, 0, len(plan.Ranges)),
		Types:   make([]SweepType, 0, len(plan.Ranges)),
		Sampled: plan.Sampled,
		Seed:    plan.Seed,
	}
	for _, r := range plan.Ranges {
		table.Params = append(table.Params, r.Param)
		table.Types = append(table.Types, r.Type)
	}
	return table
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

// runSweepRow makes one run of a sweep in ctx and tables it: the run's outputs
// and error, how long it took, and the context they are read through.
func runSweepRow(ctx *Context, bindings []SweepBinding, run SweepRun) SweepRow {
	row := SweepRow{Bindings: bindings, Context: ctx}
	started := time.Now()
	result, err := run(ctx, bindings)
	row.Elapsed = time.Since(started)
	row.Err = err
	row.Outputs, row.Verdicts = result.Outputs, result.Verdicts
	row.Subject, row.Evaluations = result.Subject, result.Evaluations
	return row
}

// SweepRunBudget is the number of runs one sweep may ask for when none is stated.
func (ctx *Context) SweepRunBudget() int64 { return ctx.maxSweepRuns }

// sweepRunLimit is the rows a sweep may make: runs when stated, else the context's.
func (ctx *Context) sweepRunLimit(runs int64) int64 {
	if runs > 0 {
		return runs
	}
	return ctx.maxSweepRuns
}

// sweepBindings is every row of a plan, in the order it is run: the cartesian
// product of the swept ranges, or one row per draw of a sampled one, at most limit.
func (ctx *Context) sweepBindings(plan SweepPlan, limit int64) ([][]SweepBinding, error) {
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
		return ctx.sampledBindings(plan, limit)
	}
	return ctx.sweptBindings(plan, limit)
}

// sweptBindings enumerates each range and takes the cartesian product, the
// first parameter varying slowest so the rows read in the order given.
func (ctx *Context) sweptBindings(plan SweepPlan, limit int64) ([][]SweepBinding, error) {
	columns := make([][]Value, len(plan.Ranges))
	total := int64(1)
	for i, r := range plan.Ranges {
		values, err := r.enumerate(ctx, limit)
		if err != nil {
			return nil, err
		}
		columns[i] = values
		if total > limit/int64(len(values)) {
			return nil, ctx.sweepBudgetError(limit)
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
func (ctx *Context) sampledBindings(plan SweepPlan, limit int64) ([][]SweepBinding, error) {
	if plan.Samples <= 0 {
		return nil, fmt.Errorf("%w: draw at least one sample, got %d", ErrSweepSamples, plan.Samples)
	}
	if plan.Samples > limit {
		return nil, ctx.sweepBudgetError(limit)
	}
	prepared := make([]sweepBounds, len(plan.Ranges))
	for i, r := range plan.Ranges {
		if r.HasStep {
			return nil, fmt.Errorf(
				"%w: sampled range %s states a step, which only a swept range advances by",
				ErrSweepSamples, r.Param,
			)
		}
		bounds, err := r.endpoints(ctx)
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

// sweepBudgetError is the refusal of a plan asking for more runs than limit allows.
func (ctx *Context) sweepBudgetError(limit int64) error {
	return fmt.Errorf("%w: at most %d run(s) per sweep%s", ErrSweepBudget, limit, ctx.raiseSweepRuns(limit))
}

// raiseSweepRuns names the variable that raises limit, when limit is the context's own.
func (ctx *Context) raiseSweepRuns(limit int64) string {
	if limit != ctx.maxSweepRuns {
		return ""
	}
	return " (raise " + MaxSweepRunsEnvVar + ")"
}

// sweepBounds is a range as arithmetic reads it: magnitudes in its first
// endpoint's unit, and whether it takes Integers, kept exactly beyond 2^53.
type sweepBounds struct {
	from, to float64
	step     float64
	isInt    bool
	intFrom  int64
	intTo    int64
	intStep  int64
	// whole reports both endpoints are whole numbers, which step by one unstated.
	whole    bool
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
		return finiteMagnitude(s.num.AsReal(), what)
	}
	q := Quantity{Num: s.num, Unit: s.unit}
	m, err := q.ConvertTo(unit)
	if err != nil {
		return 0, fmt.Errorf("%w: %s is expressed in %s, not in %s", ErrSweepRange, what, s.unit, unit)
	}
	return finiteMagnitude(m, what)
}

// finiteMagnitude refuses a magnitude no sequence of values runs between.
func finiteMagnitude(m float64, what string) (float64, error) {
	if math.IsNaN(m) || math.IsInf(m, 0) {
		return 0, fmt.Errorf("%w: %s is not a finite number", ErrSweepRange, what)
	}
	return m, nil
}

// endpoints reads a range's endpoints in the unit its first one carries and in
// the swept parameter's type, each admitted as an argument would be.
func (r SweepRange) endpoints(ctx *Context) (sweepBounds, error) {
	fromWhat, toWhat := "range start "+FormatValue(r.From), "range end "+FormatValue(r.To)
	from, err := sweepScalarOf(r.From, fromWhat)
	if err != nil {
		return sweepBounds{}, err
	}
	to, err := sweepScalarOf(r.To, toWhat)
	if err != nil {
		return sweepBounds{}, err
	}
	bounds := sweepBounds{unit: from.unit, quantity: from.quantity}
	if bounds.from, err = finiteMagnitude(from.num.AsReal(), fromWhat); err != nil {
		return sweepBounds{}, err
	}
	if bounds.to, err = to.magnitudeIn(from.unit, from.quantity, toWhat); err != nil {
		return sweepBounds{}, err
	}
	fromInt, fromWhole := r.Type.integer(from, bounds.unit, bounds.from)
	toInt, toWhole := r.Type.integer(to, bounds.unit, bounds.to)
	bounds.isInt = fromWhole && toWhole
	bounds.intFrom, bounds.intTo = fromInt, toInt
	bounds.whole = bounds.from == math.Trunc(bounds.from) && bounds.to == math.Trunc(bounds.to)
	if r.Type.Numbers == SweepIntegers {
		if !fromWhole {
			return sweepBounds{}, r.Type.notAnInteger(r.Param, fromWhat)
		}
		if !toWhole {
			return sweepBounds{}, r.Type.notAnInteger(r.Param, toWhat)
		}
	}
	if !bounds.isInt {
		if err := r.exactRealEndpoints(); err != nil {
			return sweepBounds{}, err
		}
	}
	if err := r.Type.admit(ctx, r.Param, fromWhat, bounds.value(bounds.from, bounds.intFrom)); err != nil {
		return sweepBounds{}, err
	}
	if err := r.Type.admit(ctx, r.Param, toWhat, bounds.value(bounds.to, bounds.intTo)); err != nil {
		return sweepBounds{}, err
	}
	return bounds, nil
}

// bounds reads a range a sweep steps through: its endpoints and its step, which
// only a range between whole numbers may leave unstated, stepping by one.
func (r SweepRange) bounds(ctx *Context) (sweepBounds, error) {
	bounds, err := r.endpoints(ctx)
	if err != nil {
		return sweepBounds{}, err
	}
	if !r.HasStep {
		if !bounds.whole {
			return sweepBounds{}, fmt.Errorf(
				"%w: %s=%s..%s states no step; only a range between whole numbers steps by one, one with a fractional endpoint needs `:<step>`",
				ErrSweepRange, r.Param, FormatValue(r.From), FormatValue(r.To),
			)
		}
		bounds.step, bounds.intStep = 1, 1
		if bounds.descends() {
			bounds.step, bounds.intStep = -1, -1
		}
		return bounds, nil
	}
	stepWhat := "step " + FormatValue(r.Step)
	step, err := sweepScalarOf(r.Step, stepWhat)
	if err != nil {
		return sweepBounds{}, err
	}
	if bounds.step, err = step.magnitudeIn(bounds.unit, bounds.quantity, stepWhat); err != nil {
		return sweepBounds{}, err
	}
	stepInt, stepWhole := r.Type.integer(step, bounds.unit, bounds.step)
	if r.Type.Numbers == SweepIntegers && !stepWhole {
		return sweepBounds{}, r.Type.notAnInteger(r.Param, stepWhat)
	}
	if !stepWhole {
		if bounds.isInt {
			if err := r.exactRealEndpoints(); err != nil {
				return sweepBounds{}, err
			}
		}
		if err := r.Type.exactReal(r.Param, stepWhat, r.Step); err != nil {
			return sweepBounds{}, err
		}
	}
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

// descends reports a range whose end lies below its start.
func (b sweepBounds) descends() bool {
	if b.isInt {
		return b.intTo < b.intFrom
	}
	return b.to < b.from
}

// stepsAway reports whether the step leads away from the range's end.
func (b sweepBounds) stepsAway() bool {
	if b.isInt {
		return b.intTo != b.intFrom && (b.intTo > b.intFrom) != (b.intStep > 0)
	}
	return b.to != b.from && (b.to-b.from)*b.step < 0
}

// integer is the scalar as an Integer in the range's unit: any whole number for
// a parameter taking Integers, only one written as an Integer where read as written.
func (t SweepType) integer(s sweepScalar, unit semantics.Unit, magnitude float64) (int64, bool) {
	switch t.Numbers {
	case SweepReals:
		return 0, false
	case SweepIntegers:
		if n, ok := s.exactInt(unit, magnitude); ok {
			return n, true
		}
		return (semantics.Value{Kind: semantics.ValReal, Real: magnitude}).WholeNumber()
	}
	return s.exactInt(unit, magnitude)
}

// notAnInteger refuses an endpoint or step of an Integer-typed parameter that
// is not whole.
func (t SweepType) notAnInteger(param, what string) error {
	return fmt.Errorf("%w: %s is not an Integer, which %s : %s takes",
		ErrSweepRange, what, param, symbolText(t.Declared()))
}

// exactReal refuses an endpoint or step written as an Integer no Real holds
// exactly, where the range is read as reals.
func (t SweepType) exactReal(param, what string, value Value) error {
	n, ok := sweepInteger(value)
	if !ok || realHolds(n) {
		return nil
	}
	reads := "the range is read as reals"
	if t.Numbers == SweepReals {
		reads = param + " : " + symbolText(t.Declared()) + " takes Reals"
	}
	return fmt.Errorf("%w: %s is an Integer beyond what a Real holds exactly, and %s",
		ErrSweepRange, what, reads)
}

// exactRealEndpoints refuses either endpoint an Integer no Real holds exactly.
func (r SweepRange) exactRealEndpoints() error {
	if err := r.Type.exactReal(r.Param, "range start "+FormatValue(r.From), r.From); err != nil {
		return err
	}
	return r.Type.exactReal(r.Param, "range end "+FormatValue(r.To), r.To)
}

// sweepInteger is the Integer an endpoint or step was written as, a quantity's
// magnitude included.
func sweepInteger(value Value) (int64, bool) {
	switch value.Kind {
	case ValConst:
		return value.Const.Int, value.Const.Kind == semantics.ValInt
	case ValQuantity:
		if q := value.Quantity(); q != nil && q.Num.Kind == semantics.ValInt {
			return q.Num.Int, true
		}
	}
	return 0, false
}

// realHolds reports whether a Real holds the Integer without rounding it.
func realHolds(n int64) bool {
	f := float64(n)
	return f >= math.MinInt64 && f < -math.MinInt64 && int64(f) == n
}

// admit refuses an endpoint the parameter would refuse as an argument, before
// any row runs.
func (t SweepType) admit(ctx *Context, param, what string, value Value) error {
	if t.Positive {
		if n, ok := sweepMagnitude(value); ok && n <= 0 {
			return fmt.Errorf("%w: %s is not Positive, which %s : %s takes",
				ErrSweepRange, what, param, symbolText(t.Declared()))
		}
	}
	if err := t.decl.check(ctx, &value, func() string { return what + " of " + param }); err != nil {
		return fmt.Errorf("%w: %w", ErrSweepRange, err)
	}
	return t.admitMagnitude(ctx, param, what, value)
}

// admitMagnitude refuses a quantity whose magnitude the quantity type's `num` excludes
// (classifyValue), which its dimension alone does not judge.
func (t SweepType) admitMagnitude(ctx *Context, param, what string, value Value) error {
	q := value.Quantity()
	if t.num == nil || q == nil {
		return nil
	}
	magnitude := constValue(q.Num)
	prim := ctx.model.semantics.PrimTypeOf(t.num)
	refusal := fmt.Errorf("%w: %s is %s, which num : %s of %s : %s cannot hold",
		ErrSweepRange, what, describeValue(magnitude), ctx.numTypeText(t.num, prim), param, symbolText(t.Declared()))
	if !prim.IsNumeric() {
		return refusal
	}
	for _, typ := range ctx.model.semantics.FeatureTypes(t.num) {
		verdict, err := ctx.classifyValue(declScope(t.decl.Owner), magnitude, typ, nil, byAnyType)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrSweepRange, err)
		}
		if verdict == semantics.ClassifiesNone {
			return refusal
		}
	}
	return nil
}

// numTypeText names a quantity's `num` type as a refusal reads it.
func (ctx *Context) numTypeText(num *symbols.Symbol, prim semantics.PrimType) string {
	if prim != semantics.PrimUnknown {
		return prim.String()
	}
	if types := ctx.model.semantics.FeatureTypes(num); len(types) > 0 {
		return symbolText(types[0])
	}
	return unknownText
}

// sweepMagnitude is the number a produced value is, a quantity's magnitude included.
func sweepMagnitude(value Value) (float64, bool) {
	switch value.Kind {
	case ValConst:
		return value.Const.AsReal(), value.Const.IsNumeric()
	case ValQuantity:
		if q := value.Quantity(); q != nil && q.Num.IsNumeric() {
			return q.Num.AsReal(), true
		}
	}
	return 0, false
}

// exactInt is the scalar as the Integer it was written as, expressed in the
// range's unit, where that Integer is exact.
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
func (r SweepRange) enumerate(ctx *Context, limit int64) ([]Value, error) {
	bounds, err := r.bounds(ctx)
	if err != nil {
		return nil, err
	}
	count := bounds.count()
	if count > unsignedInt(limit) {
		return nil, fmt.Errorf("%w: %s=%s..%s takes %d run(s), at most %d allowed%s",
			ErrSweepBudget, r.Param, FormatValue(r.From), FormatValue(r.To), count, limit, ctx.raiseSweepRuns(limit))
	}
	values := make([]Value, 0, count)
	for i := int64(0); i < signedInt(count); i++ {
		values = append(values, bounds.at(i))
	}
	if !bounds.isInt {
		for i := 1; i < len(values); i++ {
			at, _ := sweepMagnitude(values[i])
			if before, _ := sweepMagnitude(values[i-1]); at == before {
				return nil, fmt.Errorf("%w: %s steps by %s, finer than a Real tells apart near %s, so its rows would repeat",
					ErrSweepRange, r.Param, FormatValue(bounds.valueReal(bounds.step)), FormatValue(values[i]))
			}
		}
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
	steps := math.Floor((b.to - b.from) / b.step)
	if steps >= math.MaxInt64 {
		return math.MaxInt64
	}
	if steps <= 0 {
		return 1
	}
	// The quotient rounds either way, so the value the last step reaches decides.
	n := uint64(steps)
	switch {
	case b.within(n + 1):
		n++
	case !b.within(n):
		n--
	}
	return n + 1
}

// within reports whether the value the given number of steps from the start
// stays within the range's end, allowing a step landing on it to drift there.
func (b sweepBounds) within(steps uint64) bool {
	value := b.from + float64(steps)*b.step
	if math.IsInf(value, 0) {
		return false
	}
	// The drift is compared as the distance it is, which no endpoint overflows.
	slack := math.Min(1e-12*math.Max(math.Abs(b.to), math.Abs(value)), math.Abs(b.step)/2)
	if b.step > 0 {
		return value-b.to <= slack
	}
	return b.to-value <= slack
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
	return b.valueReal(b.from + float64(i)*b.step)
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
	u := source.Float64()
	var drawn float64
	if width := hi - lo; !math.IsInf(width, 0) {
		drawn = lo + u*width
	} else {
		// A range whose width overflows is drawn from as the two endpoints
		// weighted, which stays between them however wide they are.
		drawn = lo*(1-u) + hi*u
	}
	// Either arithmetic can round up to the end, which is not drawn.
	if drawn >= hi {
		drawn = math.Nextafter(hi, lo)
	}
	return b.valueReal(drawn)
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

// value is an endpoint as the range's kind of value: n where it takes Integers,
// the magnitude as a real otherwise.
func (b sweepBounds) value(magnitude float64, n int64) Value {
	if b.isInt {
		return b.valueInt(n)
	}
	return b.valueReal(magnitude)
}

// valueReal is a real of the range, carrying its unit.
func (b sweepBounds) valueReal(magnitude float64) Value {
	return b.carry(semantics.Value{Kind: semantics.ValReal, Real: magnitude})
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

// ResolveSweepPlan refuses a range over a parameter the target does not declare,
// its subject or one the invocation binds, and types each range by its parameter.
func (ctx *Context) ResolveSweepPlan(sym *symbols.Symbol, plan SweepPlan, positional int, named []string) (SweepPlan, error) {
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return SweepPlan{}, err
	}
	declared := shape.parameterNames()
	bound := make(map[string]bool, len(named)+positional)
	for _, name := range named {
		bound[name] = true
	}
	for i, name := range shape.positionalOrder(IsRunnableCaseSymbol(sym), bound) {
		if i >= positional {
			break
		}
		bound[name] = true
	}
	resolved := plan
	resolved.Ranges = make([]SweepRange, len(plan.Ranges))
	for i, r := range plan.Ranges {
		param := shape.parameterNamed(r.Param)
		if param == nil {
			return SweepPlan{}, fmt.Errorf("%w: %s declares no input parameter %q (it declares %v)",
				ErrSweepParameter, ctx.qualifiedSymbolName(sym), r.Param, declared)
		}
		if param.IsSubject {
			return SweepPlan{}, fmt.Errorf("%w: %s is the subject of %s, which an object binds, not a range",
				ErrSweepParameter, r.Param, ctx.qualifiedSymbolName(sym))
		}
		if bound[r.Param] {
			return SweepPlan{}, fmt.Errorf("%w: %s is both an argument of the invocation and swept",
				ErrSweepParameter, r.Param)
		}
		r.Type = ctx.sweepTypeOf(param.Decl)
		resolved.Ranges[i] = r
	}
	return resolved, nil
}

// sweepTypeOf types a range by its parameter: a scalar, or a quantity through its
// number, takes Integers or reals; any other type, and none, reads it as written.
func (ctx *Context) sweepTypeOf(decl calcMemberDecl) SweepType {
	t := SweepType{decl: decl}
	typ := t.Declared()
	if typ == nil {
		t.Untyped = true
		return t
	}
	prim := ctx.model.semantics.PrimTypeOf(typ)
	if prim == semantics.PrimUnknown {
		num, ok := ctx.quantityNumber(typ)
		if !ok {
			return t
		}
		typ, prim = num, ctx.model.semantics.PrimTypeOf(num)
		t.num = num
	}
	t.Numbers = sweepNumbersOf(prim)
	t.Positive = ctx.positiveScalar(typ)
	return t
}

// sweepNumbersOf is how a scalar lattice element counts: Naturals and Integers
// by Integers, Rationals and Reals by reals, anything else as written.
func sweepNumbersOf(prim semantics.PrimType) SweepNumbers {
	switch prim {
	case semantics.PrimNatural, semantics.PrimInteger:
		return SweepIntegers
	case semantics.PrimRational, semantics.PrimReal:
		return SweepReals
	}
	return SweepAsWritten
}

// positiveScalar reports a type that is, or specializes, ScalarValues::Positive.
func (ctx *Context) positiveScalar(typ *symbols.Symbol) bool {
	positive := ctx.librarySymbol("ScalarValues::Positive")
	return positive != nil && ctx.modelConforms(typ, positive)
}

// quantityNumber is the feature holding a scalar quantity type's magnitude, the
// `num` a quantity value's number is bound to; false for any other type.
func (ctx *Context) quantityNumber(typ *symbols.Symbol) (*symbols.Symbol, bool) {
	scalar := ctx.librarySymbol(scalarQuantityTypeFQN)
	if scalar == nil || !ctx.modelConforms(typ, scalar) {
		return nil, false
	}
	num, ok := ctx.model.semantics.LookupMember(typ, vectorQuantityNumFeature)
	if !ok || num == nil {
		return nil, false
	}
	return num, true
}

// parameterNamed is the target's parameter of that name, nil for none.
func (shape *calcShape) parameterNamed(name string) *calcParameter {
	for i := range shape.Params {
		if shape.Params[i].Name == name {
			return &shape.Params[i]
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
