package runtime

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// The model checker's view of an action's state: the moves enabled in it and the
// making of one through the `check` policy, one executor step per edge.

// moveKind classifies an enabled move by what advancing the token does.
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
	}
	return fmt.Sprintf("moveKind(%d)", int(k))
}

// enabledMove is one move of a state: one token advancing its node and, at a
// decision several of whose guards hold, the branch it takes.
type enabledMove struct {
	Token int64
	// Node is where the token sits.
	Node ast.Node
	// Branch indexes the holding branches a decision takes; -1 takes the first and reveals how many hold.
	Branch int
	// Label names the move as the trace names the token, "2@left".
	Label string
	Kind  moveKind
	// Fails is the typed error making the move raises, nil for one that advances.
	Fails error
}

func (m enabledMove) String() string {
	if m.Branch >= 0 {
		return fmt.Sprintf("%s branch %d", m.Label, m.Branch+1)
	}
	return m.Label
}

// enabledMoves lists the moves of the state in token-ID order, each as its first
// branch: the tokens able to act, and those whose parked wait fails as a typed error.
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
			Token:  id,
			Node:   t.Location,
			Branch: -1,
			Label:  tokens.label(id),
			Kind:   e.moveKindOf(t),
			Fails:  fails,
		})
	}
	slices.SortFunc(moves, func(a, b enabledMove) int { return cmp.Compare(a.Token, b.Token) })
	return moves
}

// moveKindOf classifies the move of an enabled token by its node.
func (e *ActionExecutor) moveKindOf(t Token) moveKind {
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

// settle steps a state with no move so its tokens park. Virtual time is captured,
// not explored: tokens waiting on the clock alone have it advanced to the earliest
// wait, no move of the checker's; a wait for a message nothing can post is the deadlock.
func (e *ActionExecutor) settle() error {
	run := e.ctx.scheduling()
	if run.check == nil {
		return &CheckMoveError{Branch: -1, Faced: "the run is not under the check policy"}
	}
	run.check.script.set(e, 0, -1)
	return e.advance()
}

// makeMove makes the move through the `check` policy as one executor step,
// reporting how many branches the decision it faced had (0 for none).
func (e *ActionExecutor) makeMove(m enabledMove) (branches int, err error) {
	run := e.ctx.scheduling()
	if run.check == nil {
		return 0, &CheckMoveError{Token: m.Token, Branch: m.Branch, Faced: "the run is not under the check policy"}
	}
	run.check.script.set(e, m.Token, m.Branch)
	defer run.check.script.settle()
	err = e.Step()
	if errors.Is(err, ErrNothingDue) {
		// The move parked its token on the clock; settling advances it.
		err = nil
	}
	if run.check.decided != nil {
		branches = len(run.check.decided.Alternatives)
	}
	return branches, err
}

// advanceClock moves the clock to the earliest wait as a run does, until a parked
// token can proceed; a due order among executors is a choice the policy refuses.
func (e *ActionExecutor) advanceClock() error {
	defer e.ctx.beginExecutorRun(&e.driven)()
	if err := e.ctx.driveClock(e.describeWaits(nil)); err != nil {
		return err
	}
	var progress dueProgress
	moved, err := e.awaitClock(nil, &progress)
	if err != nil {
		return err
	}
	if !moved {
		return e.deadlockError(nil)
	}
	return nil
}
