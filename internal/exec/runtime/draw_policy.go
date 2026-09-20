package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// DrawPolicy is how a run resolves the draws of the RandomFunctions library: at
// random from the seeded stream, or at a fixed point of each call's distribution.
// Weighted decisions are not durations, so they draw the same under every policy.
type DrawPolicy int

const (
	// DrawRandom draws from the seeded modeled stream, or a witness's recorded draws.
	DrawRandom DrawPolicy = iota
	// DrawMin yields the least value each call can draw.
	DrawMin
	// DrawMax yields the greatest value each call can draw.
	DrawMax
	// DrawAverage yields the mean of each call's distribution.
	DrawAverage
)

// DrawPolicyNames lists the spellings ParseDrawPolicy accepts, for usage text.
var DrawPolicyNames = []string{"random", "min", "max", "average"}

// String is the policy's spelling, as ParseDrawPolicy reads it.
func (p DrawPolicy) String() string {
	switch p {
	case DrawMin:
		return "min"
	case DrawMax:
		return "max"
	case DrawAverage:
		return "average"
	}
	return "random"
}

// Fixed reports whether the policy resolves every draw without a random stream.
func (p DrawPolicy) Fixed() bool { return p != DrawRandom }

// ErrDrawPolicy is the typed error every unreadable draw policy spelling wraps.
var ErrDrawPolicy = errors.New("invalid draw policy")

// ParseDrawPolicy reads a policy spelling: one of DrawPolicyNames.
func ParseDrawPolicy(text string) (DrawPolicy, error) {
	switch strings.TrimSpace(text) {
	case "random":
		return DrawRandom, nil
	case "min":
		return DrawMin, nil
	case "max":
		return DrawMax, nil
	case "average":
		return DrawAverage, nil
	}
	return DrawRandom, fmt.Errorf("%w: %q is not one of %s", ErrDrawPolicy, text, strings.Join(DrawPolicyNames, ", "))
}

// ErrDrawUnbounded is the typed error a fixed policy raises at a call whose
// distribution has no such point: a normal has neither a least nor a greatest value.
var ErrDrawUnbounded = errors.New("draw policy has no value for the call")

// DrawUnboundedError names the call a fixed policy could not resolve.
type DrawUnboundedError struct {
	What   string
	Policy DrawPolicy
}

func (e *DrawUnboundedError) Error() string {
	return fmt.Sprintf("%v: %s under %s: the distribution is unbounded", ErrDrawUnbounded, e.What, e.Policy)
}

// Is makes every DrawUnboundedError match ErrDrawUnbounded.
func (e *DrawUnboundedError) Is(target error) bool { return target == ErrDrawUnbounded }

// SetDrawPolicy fixes how the runs started from now on resolve their random draws;
// a run under way, paused or not, keeps the policy it started under.
func (ctx *Context) SetDrawPolicy(policy DrawPolicy) {
	ctx.drawPolicy = policy
}

// DrawPolicy is the policy the runs started from now on resolve their random draws under.
func (ctx *Context) DrawPolicy() DrawPolicy {
	return ctx.drawPolicy
}

// DrawPolicyTaken is the policy the last run's draws were resolved under: the
// witness's under a `replay` policy, whose recorded draws the run consumed, else
// the one the run started under, else the context's own before any run.
func (ctx *Context) DrawPolicyTaken() DrawPolicy {
	if w, ok := ctx.schedule.Witness(); ok {
		return w.DrawPolicy
	}
	if ctx.run != nil && ctx.run.scheduler != nil {
		return ctx.run.scheduler.draws
	}
	return ctx.drawPolicy
}

// fixedPoint is the value a fixed policy resolves the distribution to, if it has one.
func (d distribution) fixedPoint(policy DrawPolicy) (semantics.Value, bool) {
	if d.fixed == nil {
		return semantics.Value{}, false
	}
	return d.fixed(policy)
}

// admitsUnder reports whether the distribution can yield v under policy: any value
// it draws at random, or exactly the fixed point a fixed policy resolves it to.
func (d distribution) admitsUnder(policy DrawPolicy, v semantics.Value) bool {
	if !policy.Fixed() {
		return d.admits(v)
	}
	point, ok := d.fixedPoint(policy)
	return ok && point == v
}

// realPoints is a fixed function over a bounded Real distribution with the mean given.
func realPoints(lo, hi, mean float64) func(DrawPolicy) (semantics.Value, bool) {
	return func(policy DrawPolicy) (semantics.Value, bool) {
		switch policy {
		case DrawMin:
			return drawnReal(lo), true
		case DrawMax:
			return drawnReal(hi), true
		case DrawAverage:
			return drawnReal(mean), true
		}
		return semantics.Value{}, false
	}
}
