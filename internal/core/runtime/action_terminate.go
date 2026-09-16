package runtime

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// terminated unwinds a body out to the executor step within perf, the performance
// its `terminate` ends (SysML v2 §7.17.10).
type terminated struct{ perf *actionFrame }

func (t *terminated) Error() string { return "terminate " + t.perf.describe() }

// terminates reports whether err is the unwinding of a terminate ending perf.
func terminates(err error, perf *actionFrame) bool {
	var t *terminated
	return errors.As(err, &t) && t.perf == perf
}

// terminate ends the performance s names: unwinding perf's body when it runs within
// that performance, in place when it is another flow's node.
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

// terminateTarget resolves the performance s names from perf, whose body states it:
// perf itself, its parent, or the ongoing performance of a node of a flow around it.
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

// endTerminatedFor ends the performance err unwinds to at the step of token id, unless
// a body statement drives that step and the unwinding goes on to it.
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

// endAround ends perf from the token at tokenIdx running in it: the other tokens are
// dropped and this one leaves, completing perf's node; the root ends the action.
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

// endOther ends the ongoing perf a terminate outside it named: one token of it leaves,
// completing its node in the flow around it.
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

// leaveTerminated takes the token at tokenIdx out of the ended perf to perf's node in
// the flow around it and completes the node, as leaveSubflow does.
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

// dropTokensIn drops every token in perf's flow and the flows nested in it but the one
// with ID keep, lowest ID first, ending their paused work; the trace records the drops.
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
