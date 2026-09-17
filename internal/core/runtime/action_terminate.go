package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A `terminate` ends a performance before its flow completes (SysML v2 7.16): its
// tokens go with it, and the node it performs completes with its outputs as they stand.

// terminate ends the performance the statement in perf names and reports flowTerminate
// where perf stood in it, leaving the statements after it unrun up to that level.
func (e *performances) terminate(perf *actionFrame, s lower.Effect) (stmtFlow, error) {
	target, err := e.terminationTarget(perf, s)
	if err != nil {
		return flowNext, err
	}
	if err := e.owner.terminatePerformance(target); err != nil {
		return flowNext, err
	}
	if perf.standsIn(target) {
		return flowTerminate, nil
	}
	return flowNext, nil
}

// terminationTarget is the performance a terminate standing in perf ends: perf (or
// its owner, for `action t terminate;`) when it names none, else the named action's.
func (e *performances) terminationTarget(perf *actionFrame, s lower.Effect) (*actionFrame, error) {
	if s.Target == nil {
		if s.IsNode && perf.parent != nil {
			return perf.parent, nil
		}
		return perf, nil
	}
	name := ast.QualifiedText(s.Target)
	if e.ctx.model.resolver == nil || s.Scope == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnresolvedReference, name)
	}
	sym, ok := e.ctx.model.resolver.ResolveTarget(s.Scope, s.Target)
	if !ok || sym == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnresolvedReference, name)
	}
	if !isOccurrenceDecl(sym.Decl) {
		return nil, fmt.Errorf("%w: %s is not an action", ErrTerminateTarget, name)
	}
	if target := e.ongoing(e.root, sym); target != nil {
		return target, nil
	}
	return nil, fmt.Errorf("%w: %s is not being performed here", ErrTerminateTarget, name)
}

// isOccurrenceDecl reports whether decl declares something a performance can be
// of: an action or case definition, or a usage of one.
func isOccurrenceDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Usage:
		return d.Kind == ast.UsageAction || lower.IsCaseNode(d)
	case *ast.Definition:
		return d.Kind == ast.DefAction || lower.PerformsSteps(d)
	}
	return false
}

// ongoing is the unended performance of sym under root, root included: the root
// where it performs sym, else the latest performance of the node sym declares.
func (e *performances) ongoing(root *actionFrame, sym *symbols.Symbol) *actionFrame {
	if root == nil || root.ended {
		return nil
	}
	if root == e.root && e.owner.performsOwn(sym) {
		return root
	}
	if root.node != nil && root.node == sym.Decl {
		return root
	}
	for _, sub := range root.subactions {
		if found := e.ongoing(sub, sym); found != nil {
			return found
		}
	}
	return nil
}

// performsOwn reports whether sym is the action this executor performs or the
// usage it performs it for.
func (e *ActionExecutor) performsOwn(sym *symbols.Symbol) bool {
	return sym != nil && (sym == e.action || sym == e.performed)
}

// standsIn reports whether a statement running in perf stands in target: perf is
// target or a performance nested under it.
func (perf *actionFrame) standsIn(target *actionFrame) bool {
	for f := perf; f != nil; f = f.parent {
		if f == target {
			return true
		}
	}
	return false
}

// endWith marks perf and every performance under it ended by the termination of
// by, with no token left to run in their flows.
func (perf *actionFrame) endWith(by *actionFrame) {
	perf.ended, perf.terminated, perf.live = true, by, 0
	for _, sub := range perf.subactions {
		if !sub.ended {
			sub.endWith(by)
		}
	}
}

// terminatePerformance ends perf and the tokens of its flow, then completes what
// perf stood for: the executor, a body's node, or a node whose succession is taken.
func (e *ActionExecutor) terminatePerformance(perf *actionFrame) error {
	perf.endWith(perf)
	if tr := e.trace(); tr != nil {
		tr.RecordActionTerminate(perf.describe())
	}
	kept := e.dropTokensIn(perf, perf != e.root && !perf.inBody && perf.graph != nil)
	switch {
	case perf == e.root:
		if !perf.inBody {
			e.state = StateCompleted
			e.ctx.endPerformanceLife(e.occurrence)
		}
		return nil
	case perf.inBody:
		return nil
	case kept >= 0:
		e.tokens[kept].frame = perf
		return e.leaveSubflow(kept)
	}
	return e.completeTerminatedNode(perf)
}

// dropTokensIn removes the tokens running in perf's flow, cancelling paused work not
// on the stack; with keepOne, the first stays, stripped of work, and its index returns.
func (e *ActionExecutor) dropTokensIn(perf *actionFrame, keepOne bool) int {
	kept := -1
	remaining := e.tokens[:0]
	for _, token := range e.tokens {
		if !token.inFlowOf(perf) {
			remaining = append(remaining, token)
			continue
		}
		if token.body != nil && !e.ctx.active(token.body) {
			token.body.cancel(e.ctx)
		}
		token.body, token.Wait = nil, nil
		if keepOne && kept < 0 {
			kept = len(remaining)
			remaining = append(remaining, token)
		}
	}
	for i := len(remaining); i < len(e.tokens); i++ {
		e.tokens[i] = Token{}
	}
	e.tokens = remaining
	return kept
}

// completeTerminatedNode ends the paused work performing perf's node so its token
// takes the succession; work still on the stack completes as it unwinds.
func (e *ActionExecutor) completeTerminatedNode(perf *actionFrame) error {
	for i := range e.tokens {
		token := &e.tokens[i]
		if token.frame != perf.parent || token.Location != perf.node || token.body == nil {
			continue
		}
		if e.ctx.active(token.body) {
			return nil
		}
		token.body.cancel(e.ctx)
		token.body = nil
		if err := e.endPerformance(perf); err != nil {
			return err
		}
		return e.completeNode(i, perf)
	}
	return nil
}
