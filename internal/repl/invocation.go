package repl

import (
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
		inv.names = append(inv.names, b.Name)
		performers = append(performers, b.Performer...)
	}
	for _, b := range states {
		sym, err := s.exploredMachine(b.Name)
		if err != nil {
			unresolved = append(unresolved, unresolvedVerdict(b.Name, err.Error()))
			continue
		}
		inv.behaviors = append(inv.behaviors, freshBehavior{Behavior: b, sym: sym})
		inv.names = append(inv.names, b.Name)
		performers = append(performers, b.Performer...)
	}
	if len(unresolved) > 0 || len(inv.behaviors) == 0 {
		return nil, unresolved
	}
	inv.plan = s.planFresh(performers...)
	return inv, nil
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

// performer is the first object named as performing a behavior, "" for none.
func (r *freshInvocation) performer() string {
	for _, b := range r.behaviors {
		if len(b.Performer) > 0 {
			return b.Performer[0]
		}
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
	for i, exec := range inv.Actions {
		if _, err := completedActionOutcome(ctx, exec, r.names[i]); err != nil {
			return runtime.Outcome{}, err
		}
	}
	return inv.Outcome(), nil
}

// ask is the invocation's schedules as a question to the engines: how a run starts
// it, what the session has a check compare and write, and one linearization.
func (r *freshInvocation) ask(c checkSettings) (*analysis.CheckAsk, analysis.Linearization) {
	return &analysis.CheckAsk{
		Start:      r.start,
		Performer:  r.performer(),
		Diverge:    append([]string(nil), c.diverge...),
		WitnessDir: c.witnessDir,
	}, r.run
}
