package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// An invocation under check or replay moves one executor one unit at a time. The
// due order is drawn as the clock draws it when it runs what is due: among the
// executors with a move, the one drawn holds the turn until it has no move left
// at the instant, then the order is drawn again. A turn only one executor could
// take is held by none, so a state spells the same however it was reached.
// Between moves the run settles: actions park their tokens, and the clock moves
// to the earliest wait within the horizon once nothing is left at the instant.

// invocationRun is an invocation driven move by move. Its executors share one
// run: one scheduler draws every choice, and one executor completing ends nothing.
type invocationRun struct {
	ctx   *Context
	inv   *Invocation
	state *runState
	// turn is the executor the due order drew, whose moves alone are enabled while
	// it has one and another executor has one too.
	turn checkedExecutor
}

// beginInvocation starts the invocation within one run of its own, so the
// executors started, their initialization included, drive that run rather
// than one each. A starter that fails returns its error and a nil run.
func beginInvocation(ctx *Context, start Starter) (*invocationRun, error) {
	r := &invocationRun{ctx: ctx}
	defer r.enter()()
	inv, err := start(ctx)
	if err != nil {
		return nil, err
	}
	r.inv = inv
	return r, nil
}

// enter brackets one call into the invocation's run, begun at the first.
func (r *invocationRun) enter() func() {
	if r.state == nil {
		r.state = r.ctx.newRunState()
	}
	return r.ctx.enterRun(r.state)
}

// enabledMoves lists the moves of the state: the turn holder's while it has one
// and another executor has one too, else every executor's in executor order, the
// turn given up.
func (r *invocationRun) enabledMoves() []enabledMove {
	defer r.enter()()
	var all, held []enabledMove
	for _, exec := range r.inv.executors() {
		moves := exec.enabledMoves()
		if exec == r.turn {
			held = moves
		}
		all = append(all, moves...)
	}
	if len(held) == 0 || len(held) == len(all) {
		r.turn = nil
		return all
	}
	return held
}

// owners lists the executors with a move among moves, in executor order.
func owners(moves []enabledMove) []checkedExecutor {
	var execs []checkedExecutor
	for _, m := range moves {
		if !slices.Contains(execs, m.Owner) {
			execs = append(execs, m.Owner)
		}
	}
	return execs
}

// drawOwner resolves which of the executors with a move takes the turn: the only
// one where there is one, else the policy's pick, noted as a due-order choice.
func (r *invocationRun) drawOwner(execs []checkedExecutor) (int, error) {
	if len(execs) < 2 {
		if len(execs) == 1 {
			r.turn = execs[0]
		}
		return 0, nil
	}
	choice := ChoicePoint{
		Kind:         ChoiceDueOrder,
		Where:        "t=" + semantics.FormatReal(r.ctx.clock.now),
		Alternatives: executorLabels(execs),
	}
	scheduling := r.ctx.scheduling()
	pick := scheduling.choose(choice, nil)
	if err := scheduling.refusal(); err != nil {
		return 0, err
	}
	choice.Taken = pick
	r.ctx.noteChoice(choice)
	r.turn = execs[pick]
	return pick, nil
}

// stabilize settles the run to a state to move from: one with a move enabled, or
// a terminal one. Actions with nothing to do park their tokens; nothing left at
// the instant moves the clock to the earliest wait within the horizon; nothing
// left at all is terminal, or a deadlock when an action stands incomplete.
func (r *invocationRun) stabilize() error {
	for {
		if len(r.enabledMoves()) > 0 {
			return nil
		}
		if err := r.park(); err != nil {
			return err
		}
		if len(r.enabledMoves()) > 0 {
			return nil
		}
		if !r.advanceClock() {
			return r.deadlock()
		}
	}
}

// park steps every running action with no move so its tokens park: at their
// accepts, or on the clock. Under check the step is scripted to select none.
func (r *invocationRun) park() error {
	defer r.enter()()
	for _, exec := range r.inv.executors() {
		action, isAction := exec.(*ActionExecutor)
		if !isAction || (action.state != StateRunning && action.state != StateWaiting) {
			continue
		}
		if check := r.ctx.scheduling().check; check != nil {
			check.script.set(action, 0, -1, nil)
			check.begin()
		}
		if err := action.stepOne(); err != nil {
			return err
		}
	}
	return nil
}

// advanceClock moves the clock to the earliest wait ahead within the horizon,
// false when none is.
func (r *invocationRun) advanceClock() bool {
	next, ok := r.ctx.clock.NextDue()
	if !ok || next <= r.ctx.clock.now || !r.inv.Horizon.Reaches(next) {
		return false
	}
	r.ctx.clock.now = next
	r.turn = nil
	return true
}

// terminal reports whether the settled state is one the run ends in: no move,
// nothing due within the horizon, and every started action complete — a machine
// resting in a configuration nothing wakes it from is final, and so is an
// executor waiting on the clock past the horizon.
func (r *invocationRun) terminal() bool {
	return len(r.enabledMoves()) == 0 && r.deadlock() == nil
}

// deadlock is the error of the settled state with nothing left: nil where it is
// terminal, else the first incomplete executor's deadlock.
func (r *invocationRun) deadlock() error {
	for _, exec := range r.inv.executors() {
		if r.waitsPastHorizon(exec) {
			continue
		}
		if err := exec.incomplete(); err != nil {
			return err
		}
	}
	return nil
}

// waitsPastHorizon reports whether the executor has a wait on the clock the
// horizon keeps the run from reaching.
func (r *invocationRun) waitsPastHorizon(exec checkedExecutor) bool {
	for _, w := range exec.clockWaits() {
		if !r.inv.Horizon.Reaches(w.Due) {
			return true
		}
	}
	return false
}

// makeMove makes the move under the `check` policy as one atomic unit: the owner
// drawn as the due order among the executors with a move where no turn is held,
// the token and picks scripted. It reports the choice points the move drew past its picks.
func (r *invocationRun) makeMove(m enabledMove) ([]ChoicePoint, error) {
	defer r.enter()()
	check := r.ctx.scheduling().check
	if check == nil {
		return nil, &CheckMoveError{Move: m.String(), Faced: "the run is not under the check policy"}
	}
	execs := owners(r.enabledMoves())
	due := slices.Index(execs, m.Owner)
	if due < 0 {
		return nil, &CheckMoveError{Move: m.String(), Faced: "the executor has no move"}
	}
	check.script.set(m.Owner, m.Token, due, m.Picks)
	check.begin()
	defer check.script.settle()
	err := r.step(execs)
	return check.drawn, err
}

// step moves one executor one unit: the turn holder, drawn by the policy among
// those with a move where none holds it.
func (r *invocationRun) step(execs []checkedExecutor) error {
	defer r.enter()()
	pick, err := r.drawOwner(execs)
	if err != nil {
		return err
	}
	return execs[pick].stepOne()
}
