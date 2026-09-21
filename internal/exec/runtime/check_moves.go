package runtime

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// The model checker's view of an invocation's state: the moves enabled in it,
// per executor on the clock, and the making of one through the `check` policy —
// one action step, one dispatch or one do step per edge.

// moveKind classifies an enabled move by what making it does.
type moveKind int

const (
	// movePlain advances a token at a node that always can: fork, merge, final, body, nested action.
	movePlain moveKind = iota
	// moveJoin advances a token at a join whose branches have all arrived.
	moveJoin
	// moveResume goes on with a token's paused body.
	moveResume
	// moveAccept takes a message in flight, or raises the routing error its `via` port leaves.
	moveAccept
	// moveTrigger ends a time or change wait that holds, or raises the error evaluating it leaves.
	moveTrigger
	// moveDecision follows one holding guard of a decision.
	moveDecision
	// moveDispatch dispatches a state machine's next event: a change condition
	// risen, a signal in flight, or the event due at the head of its queue.
	moveDispatch
	// moveDoStep runs one due do behavior of a state machine one unit.
	moveDoStep
	// moveEntry continues a held state entry.
	moveEntry
)

func (k moveKind) String() string {
	switch k {
	case movePlain:
		return "plain"
	case moveJoin:
		return "join"
	case moveResume:
		return "resume"
	case moveAccept:
		return "accept"
	case moveTrigger:
		return "trigger"
	case moveDecision:
		return "decision"
	case moveDispatch:
		return "dispatch"
	case moveDoStep:
		return "do"
	case moveEntry:
		return "entry"
	}
	return fmt.Sprintf("moveKind(%d)", int(k))
}

// checkedExecutor is an executor the checker moves: an action or a state machine
// on the invocation's clock.
type checkedExecutor interface {
	clockWaiter
	// enabledMoves lists the moves of the executor's state, each with the picks
	// naming it among its siblings.
	enabledMoves() []enabledMove
	// stepOne makes one unit of the executor's work under the policy the run is
	// under: the move the `check` policy scripts, or the one a replay's witness fixes.
	stepOne() error
	// incomplete is the deadlock of an executor short of its end with nothing left
	// to do; nil for one at its end, or a machine at rest in a final configuration.
	incomplete() error
}

// enabledMove is one move of a state: one executor acting one unit — a token of
// an action advancing its node, a state machine dispatching or running one do
// step — and the picks resolving the choice points the move draws, in order.
type enabledMove struct {
	// Owner is the executor that moves.
	Owner checkedExecutor
	// Token is the token an action's move advances, 0 for a state machine's.
	Token int64
	// Node is where the token sits, or the state whose do behavior steps; nil for a dispatch.
	Node ast.Node
	// Picks resolve the choice points the move draws, in order; a choice past
	// them takes its first alternative and reveals its siblings.
	Picks []int
	// Label names the move as the trace names the token, "2@left", or the
	// machine's unit, "dispatch go", "do Heating".
	Label string
	Kind  moveKind
	// Fails is the typed error making the move raises, nil for one that advances.
	Fails error
}

func (m enabledMove) String() string {
	s := m.Label
	for _, pick := range m.Picks {
		s += fmt.Sprintf(" pick %d", pick+1)
	}
	return s
}

// sameUnit reports whether the two are one executor acting one unit, whatever they pick.
func (m enabledMove) sameUnit(o enabledMove) bool {
	return m.Owner == o.Owner && m.Token == o.Token && m.Kind == o.Kind && m.Node == o.Node
}

// same reports whether the two are one move: one unit taking one pick sequence.
func (m enabledMove) same(o enabledMove) bool {
	return m.sameUnit(o) && slices.Equal(m.Picks, o.Picks)
}

// enabledMoves lists the moves of the state in token-ID order, each with no pick
// yet: the tokens able to act, and those whose parked wait fails as a typed error.
func (e *ActionExecutor) enabledMoves() []enabledMove {
	defer e.ctx.beginExecutorRun(&e.driven)()
	if e.state != StateRunning && e.state != StateWaiting {
		return nil
	}
	order := e.beginStepOrder()
	tokens := e.stepCandidates(&order, oneMoveEligible)
	var moves []enabledMove
	for _, id := range tokens.ids {
		if tokens.held[id] {
			continue
		}
		ready, fails := e.readiness(id, oneMoveEligible)
		if !ready && fails == nil {
			continue
		}
		t := e.tokens[e.tokenIndex(id)]
		moves = append(moves, enabledMove{
			Owner: e,
			Token: id,
			Node:  t.Location,
			Label: tokens.label(id),
			Kind:  e.moveKindOf(t),
			Fails: fails,
		})
	}
	slices.SortFunc(moves, func(a, b enabledMove) int { return cmp.Compare(a.Token, b.Token) })
	return moves
}

// moveKindOf classifies the move of an enabled token by its node.
func (e *ActionExecutor) moveKindOf(t Token) moveKind {
	if e.dynamics != nil {
		return moveTrigger
	}
	if t.body != nil {
		return moveResume
	}
	switch node := t.Location.(type) {
	case *ast.JoinNode:
		return moveJoin
	case *ast.DecisionNode:
		return moveDecision
	case *ast.Usage:
		if accept, isAccept := e.graphOf(t.frame).Accepts[node]; isAccept {
			if accept.Trigger != nil {
				return moveTrigger
			}
			return moveAccept
		}
	}
	return movePlain
}

// stepOne makes one executor step: the scripted token's move, or — none scripted
// — a step parking the tokens at their accepts or on the clock. Nothing due is no
// error: the clock is the checker's to move.
func (e *ActionExecutor) stepOne() error {
	err := e.Step()
	if errors.Is(err, ErrNothingDue) {
		return nil
	}
	return err
}

// incomplete is the deadlock of an action started and not complete.
func (e *ActionExecutor) incomplete() error {
	if e.state == StateRunning || e.state == StateWaiting || e.state == StateSuspended {
		return e.deadlockError(nil)
	}
	return nil
}

// enabledMoves lists the moves oneUnit makes: the dispatch a closed round owes; else
// the round's do steps and an acting dispatch, picked as the step order lists them.
func (e *StateExecutor) enabledMoves() []enabledMove {
	defer e.ctx.beginExecutorRun(&e.driven)()
	if e.state != StateRunning && e.state != StateSuspended {
		return nil
	}
	if len(e.held) > 0 {
		dispatch, free := e.dispatchFree(e.dueDispatch())
		var moves []enabledMove
		if free {
			for _, move := range e.dispatchMoves(dispatch, true) {
				move.Picks = slices.Concat([]int{0}, move.Picks)
				moves = append(moves, move)
			}
		}
		for i, item := range e.held {
			picks := []int(nil)
			if free || len(e.held) > 1 {
				pick := i
				if free {
					pick++
				}
				picks = []int{pick}
			}
			moves = append(moves, enabledMove{Owner: e, Node: item.owner, Label: e.entryLabel(item.owner), Kind: moveEntry, Picks: picks})
		}
		return moves
	}
	dispatch := e.dueDispatch()
	if e.roundDone && dispatch.due {
		return e.dispatchMoves(dispatch, false)
	}
	round := e.dueRound()
	if len(round) == 0 {
		return e.dispatchMoves(dispatch, false)
	}
	if !dispatch.acts {
		return e.doMoves(round, len(round) >= 2)
	}
	moves := e.doMoves(round, true)
	for _, m := range e.dispatchMoves(dispatch, true) {
		m.Picks = slices.Concat([]int{len(round)}, m.Picks)
		moves = append(moves, m)
	}
	return moves
}

// doMoves is one do-step move per due do action, picked by its index in the round
// where the unit draws an order among them.
func (e *StateExecutor) doMoves(due []*doAction, picked bool) []enabledMove {
	moves := make([]enabledMove, 0, len(due))
	names := e.stateNames(statesOf(due))
	for i, act := range due {
		m := enabledMove{Owner: e, Node: act.state, Kind: moveDoStep, Label: "do " + names[i]}
		if picked {
			m.Picks = []int{i}
		}
		moves = append(moves, m)
	}
	return moves
}

func statesOf(acts []*doAction) []*ast.StateNode {
	states := make([]*ast.StateNode, len(acts))
	for i, act := range acts {
		states[i] = act.state
	}
	return states
}

// dispatchMoves is the dispatch due: one move, or one per event tied at the head
// picked by its place among them — among the acting ones where a step order draws it.
func (e *StateExecutor) dispatchMoves(d dueDispatch, stepOrder bool) []enabledMove {
	if !d.due {
		return nil
	}
	events, label := d.tied, d.label
	if stepOrder {
		events, label = d.among, d.step
	}
	if len(events) < 2 {
		return []enabledMove{{Owner: e, Kind: moveDispatch, Label: label}}
	}
	moves := make([]enabledMove, 0, len(events))
	for i, event := range events {
		moves = append(moves, enabledMove{Owner: e, Kind: moveDispatch, Picks: []int{i}, Label: "dispatch " + e.eventLabel(event)})
	}
	return moves
}

// stepOne makes one unit of the machine's work, which its state alone fixes
// short of the choices the policy resolves; a machine with nothing to do refuses.
func (e *StateExecutor) stepOne() error {
	var progress dueProgress
	moved, err := e.runOne(&progress)
	if err != nil {
		return err
	}
	if !moved {
		return &CheckMoveError{Move: e.dueLabel(), Faced: "the machine had nothing to do"}
	}
	return nil
}

// rest leaves a machine with no move as oneUnit leaves one with nothing to do: the
// dispatch its closed round owed was not there, so its next unit opens a round.
func (e *StateExecutor) rest() {
	e.roundDone = false
}

// incomplete is nil for a machine: one at rest in a configuration nothing wakes
// it from is final, not deadlocked.
func (e *StateExecutor) incomplete() error { return nil }
