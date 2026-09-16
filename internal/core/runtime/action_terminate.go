package runtime

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// A `terminate` ends a performance (SysML v2 §7.17.10): the immediately containing
// one for a bare `terminate;` or a terminate action usage, or the ongoing performance
// of the action node it names in a flow around it. Ending drops every token running
// in it, the paused work they hold with them, and the node completes with the
// outputs assigned so far: the parent's token takes its succession as after a
// normal completion, and the root ends the action.

// terminated unwinds a body to the level of the executor that ends perf, a
// performance the body's `terminate` named that the body itself runs within.
type terminated struct{ perf *actionFrame }

func (t *terminated) Error() string { return "terminate " + t.perf.describe() }

// terminates reports whether err is the unwinding of a terminate ending perf.
func terminates(err error, perf *actionFrame) bool {
	var t *terminated
	return errors.As(err, &t) && t.perf == perf
}

// terminate ends the performance the statement s of perf's body names: by unwinding
// the body where that is perf or one around it, in place where it is another.
func (e *performances) terminate(perf *actionFrame, s lower.Effect) error {
	target, err := e.terminateTarget(perf, s)
	if err != nil {
		return err
	}
	if target.ended {
		return fmt.Errorf("%w: %s", ErrPerformanceEnded, target.describe())
	}
	// A state behavior's or a case's own flow, run for its body, is no action performance to end.
	if target.node == nil && (target.inBody || target.label != "") {
		return fmt.Errorf("%w: 'terminate' of %s is not executable", ErrStatementNotExecutable, target.describe())
	}
	if perf.within(target) {
		return &terminated{perf: target}
	}
	return e.flow.endOther(target)
}

// terminateTarget resolves the performance s names from perf, the performance of the
// body stating it: perf for no target, else the ongoing performance of the node named,
// perf's own or one an enclosing performance holds.
func (e *performances) terminateTarget(perf *actionFrame, s lower.Effect) (*actionFrame, error) {
	switch s.Terminates {
	case lower.TerminateContaining:
		return perf, nil
	case lower.TerminateEnclosing:
		if perf.parent == nil {
			return nil, fmt.Errorf("%w: %s is no step of a flow to end", ErrTerminateTarget, perf.describe())
		}
		return perf.parent, nil
	case lower.TerminateNode:
		for f := perf; f != nil; f = f.parent {
			if f.node == s.Target {
				return f, nil
			}
			if sub, ok := f.subactions[s.Target]; ok {
				return sub, nil
			}
		}
		return nil, fmt.Errorf("%w: action node %s has no ongoing performance in a flow around %s",
			ErrTerminateTarget, ActionNodeName(s.Target), perf.describe())
	case lower.TerminateOccurrence:
		return nil, fmt.Errorf("%w: %s: 'terminate %s' names an occurrence, not an action node",
			ErrTerminateOccurrence, perf.describe(), e.ctx.bindingExprText(s.TargetExpr, s.Scope))
	}
	return nil, fmt.Errorf("%w: %s: 'terminate %s' names no action node of a flow around it and no occurrence",
		ErrTerminateTarget, perf.describe(), e.ctx.bindingExprText(s.TargetExpr, s.Scope))
}

// within reports whether f is perf or a performance nested in it.
func (f *actionFrame) within(perf *actionFrame) bool {
	for g := f; g != nil; g = g.parent {
		if g == perf {
			return true
		}
	}
	return false
}

// endTerminatedFor ends the performance a terminate unwinding the step of token id
// through err named, where the step is the outermost within it; a step a body
// statement drives inside it unwinds on to that statement.
func (e *ActionExecutor) endTerminatedFor(id int64, err error) error {
	var t *terminated
	if !errors.As(err, &t) {
		return err
	}
	idx := e.tokenIndex(id)
	if idx < 0 {
		return err
	}
	for f := e.tokens[idx].frame; f != nil; f = f.parent {
		if f.inBody {
			return err
		}
		if f == t.perf {
			return e.endAround(idx, t.perf)
		}
	}
	return fmt.Errorf("%w: token %d is not running in %s", ErrTerminateTarget, id, t.perf.describe())
}

// endAround ends perf, which the token at tokenIdx runs in: the other tokens in it
// are dropped and this one leaves, completing perf's node; the root ends the action.
func (e *ActionExecutor) endAround(tokenIdx int, perf *actionFrame) error {
	id := e.tokens[tokenIdx].ID
	e.dropTokensIn(perf, id)
	if perf == e.root {
		e.removeToken(e.tokenIndex(id))
		e.state = StateCompleted
		e.ctx.endPerformanceLife(e.occurrence)
		return nil
	}
	return e.leaveTerminated(e.tokenIndex(id), perf)
}

// endOther ends perf, which a terminate outside it named while it is ongoing: the
// token standing for it in the flow around it leaves it, completing its node.
func (e *ActionExecutor) endOther(perf *actionFrame) error {
	if perf.graph != nil && !perf.inBody {
		inside := e.tokensIn(perf)
		if len(inside) == 0 {
			return fmt.Errorf("%w: no token runs in %s", ErrTerminateTarget, perf.describe())
		}
		keep := e.tokens[inside[0]].ID
		for _, idx := range inside[1:] {
			keep = min(keep, e.tokens[idx].ID)
		}
		e.dropTokensIn(perf, keep)
		return e.leaveTerminated(e.tokenIndex(keep), perf)
	}
	// A leaf node's performance outlives a step only paused: the token at the node holds it.
	for i := range e.tokens {
		token := &e.tokens[i]
		if token.frame != perf.parent || token.Location != perf.node || token.body == nil {
			continue
		}
		token.body.end(e.ctx)
		token.body = nil
		e.dropTokensIn(perf, 0)
		if err := e.endPerformance(perf); err != nil {
			return err
		}
		return e.completeNode(i, perf)
	}
	return fmt.Errorf("%w: %s is performed by a body statement, which a terminate outside it cannot end",
		ErrTerminateTarget, perf.describe())
}

// leaveTerminated takes the token at tokenIdx out of perf's flow, ended early, to
// perf's node in the flow around it, and completes the node as leaveSubflow does.
func (e *ActionExecutor) leaveTerminated(tokenIdx int, perf *actionFrame) error {
	token := &e.tokens[tokenIdx]
	if token.body != nil {
		token.body.end(e.ctx)
		token.body = nil
	}
	for f := token.frame; f != perf; f = f.parent {
		f.ended, f.live = true, 0
	}
	perf.live = 0
	token.frame = perf.parent
	token.Location = perf.node
	token.Via = lower.ActionEdge{}
	token.Wait = nil
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeExit(ActionNodeName(perf.node))
	}
	if err := e.endPerformance(perf); err != nil {
		return err
	}
	return e.completeNode(tokenIdx, perf)
}

// dropTokensIn drops the tokens running in perf's flow and the flows nested in it,
// but the one with ID keep, lowest ID first: the work each holds paused is ended,
// the nested performances it ran in end with it, and the trace records the drops.
func (e *ActionExecutor) dropTokensIn(perf *actionFrame, keep int64) {
	var dropped []Token
	for _, idx := range e.tokensIn(perf) {
		if e.tokens[idx].ID != keep {
			dropped = append(dropped, e.tokens[idx])
		}
	}
	slices.SortFunc(dropped, func(a, b Token) int { return cmp.Compare(a.ID, b.ID) })
	for _, token := range dropped {
		if token.body != nil {
			token.body.end(e.ctx)
		}
		for f := token.frame; f != nil && f != perf; f = f.parent {
			f.ended, f.live = true, 0
		}
		e.removeToken(e.tokenIndex(token.ID))
	}
	if tr := e.trace(); tr != nil {
		tr.RecordActionTerminate(perf.describe(), dropped)
	}
}
