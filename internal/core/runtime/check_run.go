package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// An invocation under check or replay moves one executor one unit at a time,
// every executor on the clock interleaving freely at an instant. Between moves
// the run settles: actions park their tokens, and the clock moves to the earliest
// wait within the horizon once nothing is left at the instant.

// invocationRun is an invocation driven move by move.
type invocationRun struct {
	ctx *Context
	inv *Invocation
}

// enabledMoves lists the moves of the state: every executor's, in executor order.
func (r *invocationRun) enabledMoves() []enabledMove {
	var moves []enabledMove
	for _, exec := range r.inv.executors() {
		moves = append(moves, exec.enabledMoves()...)
	}
	return moves
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

// drawOwner resolves which of the executors with a move makes the next: the only
// one where there is one, else the policy's pick, noted as a due-order choice.
func (r *invocationRun) drawOwner(execs []checkedExecutor) (int, error) {
	if len(execs) < 2 {
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
// false when none is; a horizon of 0 is none.
func (r *invocationRun) advanceClock() bool {
	next, ok := r.ctx.clock.NextDue()
	if !ok || next <= r.ctx.clock.now || (r.inv.Horizon > 0 && next > r.inv.Horizon) {
		return false
	}
	r.ctx.clock.now = next
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
	if r.inv.Horizon <= 0 {
		return false
	}
	for _, w := range exec.clockWaits() {
		if w.Due > r.inv.Horizon {
			return true
		}
	}
	return false
}

// makeMove makes the move under the `check` policy as one atomic unit: the owner
// drawn as the due order among the executors with a move, the token and picks
// scripted. It reports the choice points the move drew past its picks.
func (r *invocationRun) makeMove(m enabledMove) ([]ChoicePoint, error) {
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

// step moves one executor one unit: the one the policy draws among those with a move.
func (r *invocationRun) step(execs []checkedExecutor) error {
	pick, err := r.drawOwner(execs)
	if err != nil {
		return err
	}
	return execs[pick].stepOne()
}
