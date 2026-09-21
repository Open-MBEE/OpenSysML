package runtime

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// secondFQN names the unit the clock counts in, so a duration carrying a unit
// is expressed in seconds before it is scheduled.
const secondFQN = "SI::s"

// Clock is the simulation time every executor of one context shares: the current
// instant in seconds and the executors whose waits it reads in due order.
type Clock struct {
	now     float64
	waiters []clockWaiter
	// step is the instant grid the waits come due on, in seconds; 0 is a
	// continuous clock, on which a wait comes due exactly when it ends.
	step float64
}

// ClockWait describes one wait on the clock for a view of the run.
type ClockWait struct {
	// Due is the instant the wait comes due at, in seconds.
	Due float64
	// holder is the executor waiting, what the wait itself; both are described
	// only when a view asks, so scheduling on Due does not format labels.
	holder dueHolder
	what   fmt.Stringer
}

// dueHolder is an executor that names itself in a due-order choice.
type dueHolder interface {
	dueLabel() string
}

// Holder names the executor waiting.
func (w ClockWait) Holder() string {
	if w.holder == nil {
		return ""
	}
	return w.holder.dueLabel()
}

// What describes the wait itself.
func (w ClockWait) What() string {
	if w.what == nil {
		return ""
	}
	return w.what.String()
}

// Now returns the current simulation instant, in seconds.
func (c *Clock) Now() float64 {
	return c.now
}

// Step is the step the clock advances by, in seconds: waits come due at the
// first multiple of it not before they end; 0 is a continuous clock.
func (c *Clock) Step() float64 {
	return c.step
}

// ErrClockStep is the typed error a clock step that is not a finite,
// non-negative number is refused with.
var ErrClockStep = errors.New("invalid clock step")

// CheckClockStep is ErrClockStep unless step is a finite, non-negative number of seconds.
func CheckClockStep(step float64) error {
	if math.IsNaN(step) || math.IsInf(step, 0) || step < 0 {
		return fmt.Errorf("%w: a clock steps by a finite, non-negative number of seconds, not %s", ErrClockStep, semantics.FormatReal(step))
	}
	return nil
}

// SetClockStep makes the waits set from now on come due at multiples of step
// seconds, as a simulation clock ticking by step does: a wait ending between
// two ticks comes due at the later. 0 restores the continuous clock.
func (ctx *Context) SetClockStep(step float64) error {
	if err := CheckClockStep(step); err != nil {
		return err
	}
	ctx.clock.step = step
	return nil
}

// ClockStep is the step the clock's waits come due on, 0 for a continuous clock.
func (ctx *Context) ClockStep() float64 {
	return ctx.clock.step
}

// ClockStepTaken is the step the run's waits come due on: the witness's under a
// `replay` policy, which follows the clock the witness ran on, else the clock's own.
func (ctx *Context) ClockStepTaken() float64 {
	if ctx.schedule.kind == scheduleReplay {
		return ctx.schedule.replay.witness.ClockStep
	}
	return ctx.clock.step
}

// ParseClockStep reads a clock step as `-clock-step` and a witness spell it: a
// finite, non-negative number of seconds, else ErrClockStep.
func ParseClockStep(text string) (float64, error) {
	step, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not a number of seconds", ErrClockStep, text)
	}
	if err := CheckClockStep(step); err != nil {
		return 0, err
	}
	return step, nil
}

// ClockStepParseError is a witness's clock step line that does not read as one.
type ClockStepParseError struct {
	Text   string
	Line   int
	Reason string
}

func (e *ClockStepParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%v: line %d: %q: %s", ErrClockStep, e.Line, e.Text, e.Reason)
	}
	return fmt.Sprintf("%v: %q: %s", ErrClockStep, e.Text, e.Reason)
}

func (e *ClockStepParseError) Unwrap() error { return ErrClockStep }

// onTick is the first tick of a clock stepping by step not before the instant t: t itself on a
// continuous clock, within rounding (at most a millionth of a tick) of a tick, or past what a float64 counts.
func onTick(t, step float64) float64 {
	if step == 0 {
		return t
	}
	ticks := t / step
	if math.IsInf(ticks, 0) || math.Abs(ticks) >= 1<<53 {
		return t
	}
	nearest := math.Round(ticks)
	tolerance := math.Min(1e-9*math.Max(1, math.Abs(ticks)), 1e-6)
	if math.Abs(ticks-nearest) <= tolerance {
		return nearest * step
	}
	return math.Ceil(ticks) * step
}

// Waits lists everything waiting on the clock for an instant it has not
// reached, by due instant and, at one instant, by the executors' creation order.
func (c *Clock) Waits() []ClockWait {
	var waits []ClockWait
	for _, w := range c.waiters {
		for _, wait := range w.clockWaits() {
			if wait.Due > c.now {
				waits = append(waits, wait)
			}
		}
	}
	slices.SortStableFunc(waits, func(a, b ClockWait) int { return cmp.Compare(a.Due, b.Due) })
	return waits
}

// armed lists every wait on the clock, due or not, by due instant and, at one
// instant, by the executors' creation order.
func (c *Clock) armed() []ClockWait {
	var waits []ClockWait
	for _, w := range c.waiters {
		waits = append(waits, w.armedWaits()...)
	}
	slices.SortStableFunc(waits, func(a, b ClockWait) int { return cmp.Compare(a.Due, b.Due) })
	return waits
}

// notYetDue keeps the waits, given earliest first, for instants past now.
func notYetDue(waits []ClockWait, now float64) []ClockWait {
	i := slices.IndexFunc(waits, func(w ClockWait) bool { return w.Due > now })
	if i < 0 {
		return nil
	}
	return waits[i:]
}

// NextDue returns the earliest instant a wait comes due at past the current
// one, false when nothing waits on the clock.
func (c *Clock) NextDue() (float64, bool) {
	next, found := 0.0, false
	for _, w := range c.waiters {
		for _, wait := range w.clockWaits() {
			if wait.Due > c.now && (!found || wait.Due < next) {
				next, found = wait.Due, true
			}
		}
	}
	return next, found
}

// attach makes the clock drive an executor, after those created before it.
func (c *Clock) attach(w clockWaiter) {
	c.waiters = append(c.waiters, w)
}

// detach ends the clock's driving of an executor whose run its caller is done with.
func (c *Clock) detach(w clockWaiter) {
	c.waiters = slices.DeleteFunc(c.waiters, func(x clockWaiter) bool { return x == w })
}

// forgetFinished drops the executors the clock has nothing left to drive.
func (c *Clock) forgetFinished() {
	c.waiters = slices.DeleteFunc(c.waiters, clockWaiter.finished)
}

// Clock returns the simulation clock every executor of this context shares.
func (ctx *Context) Clock() *Clock {
	return &ctx.clock
}

// ClockValue is the clock's instant now as a value: a duration in seconds, or a
// bare number where no library reduces the second.
func (ctx *Context) ClockValue() Value {
	return ctx.instantValue(ctx.clock.Now())
}

// instantValue is the clock's instant t as a duration quantity, or a bare number
// where no library reduces the second.
func (ctx *Context) instantValue(t float64) Value {
	second, err := ctx.clockUnit()
	if err != nil {
		return realConst(t)
	}
	return NewQuantityValue(&Quantity{Num: semantics.Value{Kind: semantics.ValReal, Real: t}, Unit: second})
}

// InstantValue is instant t as the runtime reports the clock: a quantity in
// seconds when the library defines them, otherwise a bare real.
func (ctx *Context) InstantValue(t float64) Value {
	return ctx.instantValue(t)
}

// ClockMagnitude reads a value as a number of clock units: a bare number is one
// already, a quantity is converted from its unit; what names it in errors.
func (ctx *Context) ClockMagnitude(val Value, what string) (float64, error) {
	return ctx.timeMagnitude(val, what)
}

// dueInstant is when a time trigger comes due: `after d` counts from now (a
// negative delay is refused); `at t` is taken as read, one already past due now.
// Either must be finite, and so must the instant a delay leads to.
func (ctx *Context) dueInstant(t *ast.TimeEvent, val Value, what string) (float64, error) {
	magnitude, err := ctx.timeMagnitude(val, what)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(magnitude) {
		return 0, fmt.Errorf("%w: %s is not a number", ErrNegativeDuration, what)
	}
	if math.IsInf(magnitude, 0) {
		return 0, fmt.Errorf("%w: %s is infinite", ErrNegativeDuration, what)
	}
	if t.Absolute {
		if magnitude <= ctx.clock.now {
			return ctx.clock.now, nil
		}
		return ctx.clock.tickOf(magnitude, ctx.ClockStepTaken(), what)
	}
	if magnitude < 0 {
		return 0, fmt.Errorf("%w: %s %s is negative", ErrNegativeDuration, what, semantics.FormatReal(magnitude))
	}
	due, err := ctx.clock.instantAfter(magnitude, what)
	if err != nil {
		return 0, err
	}
	return ctx.clock.tickOf(due, ctx.ClockStepTaken(), what)
}

// tickOf is the tick a wait for the finite instant due comes due on under step; one
// past the last instant a float64 holds is refused, so the clock stays finite.
func (c *Clock) tickOf(due, step float64, what string) (float64, error) {
	tick := onTick(due, step)
	if math.IsInf(tick, 0) {
		return 0, fmt.Errorf("%w: %s at t=%s comes due on a tick of the clock stepping by %s past the last instant the clock can hold",
			ErrNegativeDuration, what, semantics.FormatReal(due), semantics.FormatReal(step))
	}
	return tick, nil
}

// instantAfter is the instant a finite, non-negative duration from now leads to;
// one past the last instant a float64 holds is refused, so the clock stays finite.
func (c *Clock) instantAfter(duration float64, what string) (float64, error) {
	due := c.now + duration
	if math.IsInf(due, 0) {
		return 0, fmt.Errorf("%w: %s %s from t=%s leads past the last instant the clock can hold",
			ErrNegativeDuration, what, semantics.FormatReal(duration), semantics.FormatReal(c.now))
	}
	return due, nil
}

// timeMagnitude reads a time trigger's duration or instant as a number of clock
// units: a bare number is already one, a quantity is converted from its unit.
func (ctx *Context) timeMagnitude(val Value, what string) (float64, error) {
	switch val.Kind {
	case ValConst:
		switch val.Const.Kind {
		case semantics.ValInt:
			return float64(val.Const.Int), nil
		case semantics.ValReal:
			return val.Const.Real, nil
		default:
			return 0, fmt.Errorf("%s must be numeric, got %v", what, val.Const.Kind)
		}
	case ValQuantity:
		return ctx.durationInClockUnits(val.Quantity(), what)
	default:
		return 0, fmt.Errorf("%s must be constant, got %v", what, val.Kind)
	}
}

// judgeTimeTriggerType refuses, before evaluating it, the trigger argument
// validation refuses; one the declarations leave open is left to its value.
func (ctx *Context) judgeTimeTriggerType(scope *symbols.Scope, t *ast.TimeEvent) error {
	c := ctx.model.semantics.TimeEventConforms(scope, t)
	if !c.Known || c.Holds {
		return nil
	}
	keyword := "after"
	if t.Absolute {
		keyword = "at"
	}
	return fmt.Errorf("%w: `%s %s` must be a %s, found %s",
		ErrTimeTriggerType, keyword, ctx.bindingExprText(t.Duration, scope), semantics.TimeEventType(t), c.Found)
}

// durationInClockUnits expresses a quantity in the clock's unit, reporting a
// quantity that does not measure time as the dimension error it is.
func (ctx *Context) durationInClockUnits(q *Quantity, what string) (float64, error) {
	second, err := ctx.clockUnit()
	if err != nil {
		return 0, fmt.Errorf("%s %s: %w", what, q, err)
	}
	if !q.Unit.Term.Commensurable(second.Term) {
		return 0, fmt.Errorf("%w: %s %s is not a time: %s does not measure a duration",
			ErrIncommensurableUnits, what, q, q.Unit)
	}
	magnitude, err := q.ConvertTo(second)
	if err != nil {
		return 0, fmt.Errorf("%s %s: %w", what, q, err)
	}
	return magnitude, nil
}

// clockUnit is the second as the Quantities and Units library reduces it, so a
// duration converts by the same reduction every other quantity uses.
func (ctx *Context) clockUnit() (Unit, error) {
	if ctx.model.resolver == nil || ctx.model.resolver.Index() == nil {
		return Unit{}, fmt.Errorf("%w: no library to reduce %s in", semantics.ErrNotAUnit, secondFQN)
	}
	matches := ctx.model.resolver.Index().LookupQualified(secondFQN)
	if len(matches) != 1 {
		return Unit{}, fmt.Errorf("%w: %s names %d elements, so no clock unit is determined",
			semantics.ErrNotAUnit, secondFQN, len(matches))
	}
	term, err := ctx.model.semantics.UnitTermOf(matches[0])
	if err != nil {
		return Unit{}, err
	}
	return Unit{Text: "s", Product: semantics.NamedUnitProduct(matches[0], "s", false), Term: term}, nil
}
