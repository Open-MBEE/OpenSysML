package runtime

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// terminated unwinds a body out to the executor step within perf, the performance
// its `terminate` ends (SysML v2 §7.17.10); then are the performances the same
// statement named after perf, ended in place once perf has, in that order.
type terminated struct {
	perf *actionFrame
	then []*actionFrame
}

func (t *terminated) Error() string { return "terminate " + t.perf.describe() }

// terminates reports whether err is the unwinding of a terminate ending perf.
func terminates(err error, perf *actionFrame) bool {
	var t *terminated
	return errors.As(err, &t) && t.perf == perf
}

// terminate ends the performances s names, the earliest begun first: in place for each
// that is another flow's node, unwinding perf's body at the one it runs within — the
// later ones then end where the unwinding is caught, so the order named holds.
func (e *performances) terminate(perf *actionFrame, s lower.Effect) error {
	targets, err := e.terminateTargets(perf, s)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if target.ended {
			return fmt.Errorf("%w: %s", ErrPerformanceEnded, target.describe())
		}
		if target.node == nil && !e.owner.endsOwn() {
			return fmt.Errorf("%w: 'terminate' of %s is not executable", ErrStatementNotExecutable, target.describe())
		}
	}
	for i, target := range targets {
		if perf.nestedIn(target) {
			return &terminated{perf: target, then: targets[i+1:]}
		}
		if err := e.flow.endOther(target); err != nil {
			return err
		}
	}
	return nil
}

// terminatedUsage goes on from a terminate action usage's body ending perf, the usage's
// own performance, to the terminate the usage stands for; any other err is returned as is.
func (e *performances) terminatedUsage(perf *actionFrame, graph *lower.ActionGraph, err error) error {
	if !terminates(err, perf) {
		return err
	}
	s, ok := graph.TerminateUsage(perf.node)
	if !ok {
		return err
	}
	return e.terminate(perf, s)
}

// terminateTargets resolves the performances s names from perf, whose body states it:
// perf itself, its parent, or the ongoing performances of a node of a flow around it.
func (e *performances) terminateTargets(perf *actionFrame, s lower.Effect) ([]*actionFrame, error) {
	switch s.Terminates {
	case lower.TerminateContaining:
		return []*actionFrame{perf}, nil
	case lower.TerminateEnclosing:
		if perf.parent == nil {
			return nil, fmt.Errorf("%w: %s is no step of a flow to end", ErrTerminateTarget, perf.describe())
		}
		return []*actionFrame{perf.parent}, nil
	case lower.TerminateNode:
		for f := perf; f != nil; f = f.parent {
			if f.node == s.Target {
				return e.ongoingWith(f, s.Target), nil
			}
			if latest, ok := f.subactions[s.Target]; ok {
				if ongoing := e.flow.ongoing(f, s.Target); len(ongoing) > 0 {
					return ongoing, nil
				}
				return []*actionFrame{latest}, nil
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

// ongoingWith returns the ongoing performances of node in the flow of within, a performance
// of node itself, which is among them: the performance running the statement is never left out.
func (e *performances) ongoingWith(within *actionFrame, node ast.Node) []*actionFrame {
	if within.parent == nil {
		return []*actionFrame{within}
	}
	ongoing := e.flow.ongoing(within.parent, node)
	if !slices.Contains(ongoing, within) {
		ongoing = append([]*actionFrame{within}, ongoing...)
	}
	return ongoing
}

// ongoing returns the performances of node in parent's flow still running: parent holds
// the latest, tokens run in or hold paused the earlier ones. The earliest begun comes
// first, those begun together in the order of the lowest token ID holding them.
func (e *ActionExecutor) ongoing(parent *actionFrame, node ast.Node) []*actionFrame {
	var found []*actionFrame
	holder := make(map[*actionFrame]int64)
	add := func(f *actionFrame, token int64) {
		if f == nil || f.parent != parent || f.node != node || f.ended {
			return
		}
		if held, ok := holder[f]; !ok || token < held {
			holder[f] = token
		}
		if !slices.Contains(found, f) {
			found = append(found, f)
		}
	}
	add(parent.subactions[node], math.MaxInt64)
	for _, token := range e.tokens {
		for f := token.frame; f != nil; f = f.parent {
			add(f, token.ID)
		}
		for _, f := range token.performed() {
			add(f, token.ID)
		}
	}
	slices.SortFunc(found, func(a, b *actionFrame) int {
		return cmp.Or(cmp.Compare(a.began, b.began), cmp.Compare(holder[a], holder[b]))
	})
	return found
}

// performed returns the performances the token's paused step holds: the node it steps,
// then each node a body statement of it was performing when it paused, outermost first.
func (t Token) performed() []*actionFrame {
	if t.body == nil {
		return nil
	}
	var held []*actionFrame
	if w, ok := t.body.work.(*usageWork); ok {
		held = append(held, w.perf)
	}
	cursor := t.body.cursor
	for i := len(cursor) - 1; i >= 0; i-- {
		if f, ok := cursor[i].(*performFrame); ok && f.perf != nil {
			held = append(held, f.perf)
		}
	}
	return held
}

// nestedIn reports whether f is perf or a performance nested in it.
func (f *actionFrame) nestedIn(perf *actionFrame) bool {
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
	t := unwound(err)
	if t == nil {
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
			if err := e.endAround(idx, t.perf); err != nil {
				return err
			}
			return e.endAlongside(t)
		}
	}
	return fmt.Errorf("%w: token %d is not running in %s", ErrTerminateTarget, id, t.perf.describe())
}

// endAlongside ends in place, in the order named, the performances the terminate t
// unwound for named after the one it ended.
func (e *ActionExecutor) endAlongside(t *terminated) error {
	for _, target := range t.then {
		if err := e.endOther(target); err != nil {
			return err
		}
	}
	return nil
}

// unwound returns the terminate err unwinds for, nil for any other error or none.
func unwound(err error) *terminated {
	var t *terminated
	if errors.As(err, &t) {
		return t
	}
	return nil
}

// endAround ends perf from the token at tokenIdx running in it: the other tokens are
// dropped and this one leaves, completing perf's node; the root ends the action.
func (e *ActionExecutor) endAround(tokenIdx int, perf *actionFrame) error {
	id := e.tokens[tokenIdx].ID
	e.dropTokensIn(perf, id)
	if perf == e.root {
		e.removeToken(e.tokenIndex(id))
		e.state = StateTerminated
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
	// A leaf node's performance outlives a step only paused: the token at the node holds
	// it, stepping the node or performing it from a statement of the body it runs.
	for i := range e.tokens {
		token := &e.tokens[i]
		if token.body == nil {
			continue
		}
		if w, ok := token.body.work.(*usageWork); ok && w.perf == perf {
			token.body.end(e.ctx)
			token.body = nil
			e.dropTokensIn(perf, 0)
			if err := e.endPerformance(perf); err != nil {
				return err
			}
			return e.completeNode(i, perf)
		}
		if token.body.endPerformed(e.ctx, perf) {
			e.dropTokensIn(perf, 0)
			perf.live = 0
			return e.endPerformance(perf)
		}
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
