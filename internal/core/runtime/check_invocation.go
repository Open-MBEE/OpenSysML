package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Invocation is what one check searches: the behaviors started on one clock, in
// the order started, and the horizon the clock runs to. The objects the started
// behaviors materialize bring their own behaviors onto the clock.
type Invocation struct {
	Actions []*ActionExecutor
	States  []*StateExecutor
	// Names are the names the behaviors' observables are reported under in a joint
	// outcome, actions first; the behaviors' own names when absent.
	Names   []string
	Horizon Horizon
}

// Horizon is the instant a check runs the clock to: the zero Horizon is none,
// and one at t=0 dispatches what is due at the start alone.
type Horizon struct {
	at      float64
	bounded bool
}

// HorizonAt is the horizon at instant t.
func HorizonAt(t float64) Horizon { return Horizon{at: t, bounded: true} }

// Bounded is the instant the horizon stops the clock at, false for none.
func (h Horizon) Bounded() (at float64, ok bool) { return h.at, h.bounded }

// Reaches reports whether the instant lies within the horizon.
func (h Horizon) Reaches(t float64) bool { return !h.bounded || t <= h.at }

// Starter builds and starts the behaviors a check runs, in the context given; a
// replay starts the same invocation the same way.
type Starter func(*Context) (*Invocation, error)

// ErrNothingStarted is the typed error a check whose starter started no behavior,
// or one in another context, wraps.
var ErrNothingStarted = errors.New("the invocation started nothing to check")

// started checks the invocation runs in ctx: at least one behavior, each started there.
func (inv *Invocation) started(ctx *Context) error {
	if len(inv.Actions) == 0 && len(inv.States) == 0 {
		return ErrNothingStarted
	}
	for _, exec := range inv.Actions {
		if exec.ctx != ctx {
			return fmt.Errorf("%w: action %s runs in another context", ErrNothingStarted, symbolText(exec.action))
		}
	}
	for _, exec := range inv.States {
		if exec.ctx != ctx {
			return fmt.Errorf("%w: state machine %s runs in another context", ErrNothingStarted, symbolText(exec.stateMachine))
		}
	}
	return nil
}

// Context is the context the started behaviors run in, nil for an empty invocation.
func (inv *Invocation) Context() *Context {
	if len(inv.Actions) > 0 {
		return inv.Actions[0].ctx
	}
	if len(inv.States) > 0 {
		return inv.States[0].ctx
	}
	return nil
}

// Release ends every started executor's run.
func (inv *Invocation) Release() {
	for _, exec := range inv.Actions {
		exec.Release()
	}
	for _, exec := range inv.States {
		exec.Release()
	}
}

// Performer is the object the started behaviors perform on: the first started
// behavior's, nil when it performs on none.
func (inv *Invocation) Performer() *Instance {
	if len(inv.Actions) > 0 {
		return inv.Actions[0].self
	}
	if len(inv.States) > 0 {
		return inv.States[0].self
	}
	return nil
}

// Snapshot captures every executor on the clock along with the context's state.
func (inv *Invocation) Snapshot() (*Snapshot, error) {
	return inv.Context().snapshotWith(slices.Clone(inv.Actions), slices.Clone(inv.States))
}

// Completed reports whether every started action reached its end.
func (inv *Invocation) Completed() bool {
	for _, exec := range inv.Actions {
		if exec.state != StateCompleted {
			return false
		}
	}
	return true
}

// names are the names the behaviors' observables are reported under, in order.
func (inv *Invocation) names() []string {
	n := len(inv.Actions) + len(inv.States)
	if len(inv.Names) == n {
		return inv.Names
	}
	names := make([]string, 0, n)
	for _, exec := range inv.Actions {
		names = append(names, symbolText(exec.action))
	}
	for _, exec := range inv.States {
		names = append(names, symbolText(exec.stateMachine))
	}
	return names
}

// joint reports an invocation of several behaviors, whose observables go under their names.
func (inv *Invocation) joint() bool {
	return len(inv.Actions)+len(inv.States) > 1
}

// prefixes are what each behavior's observables are named under, actions first:
// `<name>.` in a joint invocation, nothing in a single one.
func (inv *Invocation) prefixes() []string {
	names := inv.names()
	prefixes := make([]string, len(names))
	if inv.joint() {
		for i, name := range names {
			prefixes[i] = name + "."
		}
	}
	return prefixes
}

// behaviorOf is the behavior an observable's name belongs to, by index among
// actions then states, and the name under its prefix; -1 for none. A joint
// invocation's names are `<name>.<feature>` or `<name> finalState`; a single one's are bare.
func (inv *Invocation) behaviorOf(prefixes []string, name string) (int, string) {
	if !inv.joint() {
		return 0, name
	}
	for i, prefix := range prefixes {
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			return i, rest
		}
		if name == finalStateKey(prefix) {
			return i, ""
		}
	}
	return -1, ""
}

// Outcome is what the started behaviors came to: one behavior's own outcome, the
// joint outcome of several, each one's observables under its name.
func (inv *Invocation) Outcome() Outcome {
	ctx := inv.Context()
	outcomes := make([]Outcome, 0, len(inv.Actions)+len(inv.States))
	for _, exec := range inv.Actions {
		outcomes = append(outcomes, ctx.ActionOutcome(exec.Results()))
	}
	for _, exec := range inv.States {
		outcomes = append(outcomes, exec.Outcome())
	}
	if len(outcomes) == 1 {
		return outcomes[0]
	}
	return ctx.JointOutcome(inv.names(), outcomes)
}

// executors lists every executor the check moves, in canonical order: the started
// behaviors as started, then the objects' behaviors in the order attached.
func (inv *Invocation) executors() []checkedExecutor {
	ctx := inv.Context()
	execs := make([]checkedExecutor, 0, len(inv.Actions)+len(inv.States)+len(ctx.objectBehaviors))
	for _, exec := range inv.Actions {
		execs = append(execs, exec)
	}
	for _, exec := range inv.States {
		execs = append(execs, exec)
	}
	for _, behavior := range ctx.objectBehaviors {
		switch {
		case behavior.State != nil && !slices.Contains(inv.States, behavior.State):
			execs = append(execs, behavior.State)
		case behavior.Action != nil && !slices.Contains(inv.Actions, behavior.Action):
			execs = append(execs, behavior.Action)
		}
	}
	return execs
}

// executorLabels names the executors as a due-order choice lists them, a label
// two share numbered so a witness names one.
func executorLabels(execs []checkedExecutor) []string {
	labels := make([]string, len(execs))
	counts := make(map[string]int, len(execs))
	for i, exec := range execs {
		label := exec.dueLabel()
		counts[label]++
		if n := counts[label]; n > 1 {
			label += " #" + strconv.Itoa(n)
		}
		labels[i] = label
	}
	return labels
}
