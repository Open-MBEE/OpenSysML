package repl

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// freshBehavior is one behavior a fresh run starts: the action or machine its
// name resolved to, and the object performing it when one is named.
type freshBehavior struct {
	Behavior
	sym    *symbols.Symbol
	action bool
}

// freshInvocation is the behaviors named, run together on a fresh context: what
// a check searches every schedule of, and what one linearization of it runs. The
// horizon, when set, is how far the shared clock moves; without one each behavior
// runs to completion, a machine until nothing more is due.
type freshInvocation struct {
	behaviors []freshBehavior
	names     []string
	plan      *freshPlan
	horizon   *float64
}

// resolveInvocation resolves the behaviors named, actions then machines, while
// the session's state is held; unresolved reports each name that did not resolve.
func (s *Session) resolveInvocation(actions, states []Behavior, horizon *float64) (inv *freshInvocation, unresolved []Verdict) {
	inv = &freshInvocation{horizon: horizon}
	var performers []string
	for _, b := range actions {
		sym, err := s.exploredAction(b.Name)
		if err != nil {
			unresolved = append(unresolved, unresolvedVerdict(b.Name, err.Error()))
			continue
		}
		inv.behaviors = append(inv.behaviors, freshBehavior{Behavior: b, sym: sym, action: true})
		performers = append(performers, b.Performer...)
	}
	for _, b := range states {
		sym, err := s.exploredMachine(b.Name)
		if err != nil {
			unresolved = append(unresolved, unresolvedVerdict(b.Name, err.Error()))
			continue
		}
		inv.behaviors = append(inv.behaviors, freshBehavior{Behavior: b, sym: sym})
		performers = append(performers, b.Performer...)
	}
	if len(unresolved) > 0 || len(inv.behaviors) == 0 {
		return nil, unresolved
	}
	inv.names = jointNames(inv.behaviors)
	inv.plan = s.planFresh(performers...)
	return inv, nil
}

// jointNames name the behaviors as the verdict and a joint outcome tell them
// apart: bare when they perform on one object or none, with the object each
// performs on as named otherwise, and numbered `#n` when still the same.
func jointNames(behaviors []freshBehavior) []string {
	names := make([]string, len(behaviors))
	apart := len(distinctPerformers(behaviors)) > 1
	for i, b := range behaviors {
		names[i] = b.Name
		if apart && len(b.Performer) > 0 {
			names[i] += " " + b.Performer[0]
		}
	}
	counts := make(map[string]int, len(names))
	for _, name := range names {
		counts[name]++
	}
	seen := make(map[string]int, len(names))
	for i, name := range names {
		if counts[name] > 1 {
			seen[name]++
			names[i] = fmt.Sprintf("%s #%d", name, seen[name])
		}
	}
	return names
}

// distinctPerformers are the objects named as performing the behaviors, each once.
func distinctPerformers(behaviors []freshBehavior) []string {
	var performers []string
	for _, b := range behaviors {
		if len(b.Performer) > 0 && !slices.Contains(performers, b.Performer[0]) {
			performers = append(performers, b.Performer[0])
		}
	}
	return performers
}

// subject names the invocation as verdicts do: the behaviors in start order.
func (r *freshInvocation) subject() string {
	return strings.Join(r.names, ", ")
}

// label is the subject with what it is: an action, a state machine, or several
// behaviors on one clock.
func (r *freshInvocation) label() string {
	switch {
	case len(r.behaviors) > 1:
		return "Behaviors " + r.subject()
	case r.behaviors[0].action:
		return "Action " + r.subject()
	default:
		return "State machine " + r.subject()
	}
}

// singleAction reports whether the invocation is one action alone: what the
// symbolic engine decides.
func (r *freshInvocation) singleAction() bool {
	return len(r.behaviors) == 1 && r.behaviors[0].action
}

// performer is the one object named as performing the behaviors, "" for none or
// for several.
func (r *freshInvocation) performer() string {
	if performers := distinctPerformers(r.behaviors); len(performers) == 1 {
		return performers[0]
	}
	return ""
}

// start starts every behavior on ctx: the invocation a check searches the
// schedules of, on the one clock the behaviors share.
func (r *freshInvocation) start(ctx *runtime.Context) (*runtime.Invocation, error) {
	objects := r.plan.bind(ctx)
	inv := &runtime.Invocation{}
	for _, b := range r.behaviors {
		if b.action {
			exec, err := freshAction(objects, b.sym, b.Performer)
			if err != nil {
				return nil, err
			}
			inv.Actions = append(inv.Actions, exec)
			continue
		}
		exec, err := freshMachine(objects, b.sym, b.Name, b.Performer)
		if err != nil {
			return nil, err
		}
		inv.States = append(inv.States, exec)
	}
	if len(r.behaviors) > 1 {
		inv.Names = r.names
		inv.PerformerNames = make([]string, len(r.behaviors))
		for i, b := range r.behaviors {
			if len(b.Performer) > 0 {
				inv.PerformerNames[i] = b.Performer[0]
			}
		}
	}
	if r.horizon != nil {
		inv.Horizon = runtime.HorizonAt(*r.horizon)
	}
	return inv, nil
}

// run is one linearization of the invocation: the shared clock advanced to the
// horizon, or each behavior run to completion in start order. An action that
// stopped short is an error, as the prompt's run reports it.
func (r *freshInvocation) run(ctx *runtime.Context) (runtime.Outcome, error) {
	inv, err := r.start(ctx)
	if err != nil {
		return runtime.Outcome{}, err
	}
	switch {
	case r.horizon != nil:
		if _, err := ctx.Advance(*r.horizon); err != nil {
			return runtime.Outcome{}, err
		}
	default:
		for _, exec := range inv.Actions {
			if err := exec.RunToCompletion(); err != nil {
				return runtime.Outcome{}, err
			}
		}
		for _, exec := range inv.States {
			if err := exec.RunToCompletion(); err != nil {
				return runtime.Outcome{}, err
			}
		}
	}
	actions := 0
	for _, b := range r.behaviors {
		if !b.action {
			continue
		}
		if _, err := completedActionOutcome(ctx, inv.Actions[actions], b.Name); err != nil {
			return runtime.Outcome{}, err
		}
		actions++
	}
	return inv.Outcome(), nil
}

// startAction starts the invocation when it is one action alone, as the symbolic
// engine starts one; nil otherwise.
func (r *freshInvocation) startAction() analysis.Start {
	if !r.singleAction() {
		return nil
	}
	return func(ctx *runtime.Context) (*runtime.ActionExecutor, error) {
		inv, err := r.start(ctx)
		if err != nil {
			return nil, err
		}
		return inv.Actions[0], nil
	}
}

// asks is the invocation's schedules as questions to the engines: how a run starts
// it, what the session has a check compare and write, and one linearization. The
// symbolic ask is made for one action alone, with the inputs the session frees.
func (r *freshInvocation) asks(c checkSettings) checkAsks {
	asks := checkAsks{
		check: &analysis.CheckAsk{
			Start:      r.start,
			Performer:  r.performer(),
			Diverge:    append([]string(nil), c.diverge...),
			WitnessDir: c.witnessDir,
		},
		run: r.run,
	}
	if start := r.startAction(); start != nil {
		asks.holds = &analysis.HoldsAsk{
			Behavior:   r.behaviors[0].sym,
			Start:      start,
			Performer:  r.performer(),
			Inputs:     append([]string(nil), c.inputs...),
			WitnessDir: c.witnessDir,
		}
	}
	return asks
}
