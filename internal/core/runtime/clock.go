package runtime

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// secondFQN names the unit the clock counts in, so a duration carrying a unit
// is expressed in seconds before it is scheduled.
const secondFQN = "SI::s"

// Clock is the simulation time every executor of one context shares: the current
// instant in seconds and the executors whose waits it reads in due order.
type Clock struct {
	now     float64
	waiters []clockWaiter
}

// ClockWait describes one wait on the clock for a view of the run.
type ClockWait struct {
	// Due is the instant the wait comes due at, in seconds.
	Due float64
	// Holder names the executor waiting, What the wait itself.
	Holder, What string
}

// Now returns the current simulation instant, in seconds.
func (c *Clock) Now() float64 {
	return c.now
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
		return math.Max(magnitude, ctx.clock.now), nil
	}
	if magnitude < 0 {
		return 0, fmt.Errorf("%w: %s %s is negative", ErrNegativeDuration, what, semantics.FormatReal(magnitude))
	}
	return ctx.clock.instantAfter(magnitude, what)
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
