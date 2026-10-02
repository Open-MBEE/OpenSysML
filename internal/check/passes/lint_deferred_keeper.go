package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// DeferredKeeperPass warns when a state annotated `MigrationMetadata::DeferredEvent`
// has, at the root of its do action, an accept typed by a signal the annotation
// defers that does not carry `MigrationMetadata::DeferredKeeper`. That is the
// keeping loop of the standard deferred-signal encoding as it was written before
// the marker existed: the runtime knows the keeping accept by the marker alone,
// so without it the loop is an ordinary accept of the signal, which consumes an
// occurrence instead of keeping it.
type DeferredKeeperPass struct{}

// Level reports the name-resolution level: the annotations' types and the
// signals' names are all it reads.
func (DeferredKeeperPass) Level() PassLevel { return LevelNameResolution }

// Run checks the deferring states of one workspace document.
func (DeferredKeeperPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil || ctx.Index.IsLibraryDocument(name) {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &deferredKeeperLint{ctx: ctx, resolver: ctx.Resolver()}
	for _, at := range kit.ScopedNodes(ctx, rootScope) {
		if u, ok := at.Node.(*ast.Usage); ok && u.Kind == ast.UsageState {
			c.checkState(at.Scope, u)
		}
	}
	return c.diags
}

type deferredKeeperLint struct {
	ctx      *Context
	resolver *resolve.Resolver
	diags    []diag.Diagnostic
}

// checkState reports the unmarked accepts of each deferred signal at the root
// of the do action of the state usage u, written in scope, when no accept of
// that signal there is marked as its keeper.
func (c *deferredKeeperLint) checkState(scope *symbols.Scope, u *ast.Usage) {
	deferred := lower.DeferredSignals(c.resolver, scope, u)
	if len(deferred) == 0 {
		return
	}
	body := scopeOwned(scope, u)
	for _, member := range u.Members {
		do, ok := memberNode(member).(*ast.DoMember)
		if !ok {
			continue
		}
		for _, action := range do.Actions {
			if performed, ok := memberNode(action).(*ast.Usage); ok {
				c.checkDoAction(scopeOwned(body, performed), performed, deferred)
			}
		}
	}
}

// rootAccept is an accept node at the root of a do action with the signal its
// payload is typed by.
type rootAccept struct {
	node   *ast.Usage
	signal *symbols.Symbol
}

// checkDoAction reports, for each of deferred, the accepts of the signal among
// the root nodes of the performed do action when none of them is marked.
func (c *deferredKeeperLint) checkDoAction(scope *symbols.Scope, performed *ast.Usage, deferred []lower.DeferredSignal) {
	var accepts []rootAccept
	for _, m := range performed.Members {
		node, ok := memberNode(m).(*ast.Usage)
		if !ok || node.Kind != ast.UsageAction {
			continue
		}
		if signal := c.acceptedSignal(scope, node); signal != nil {
			accepts = append(accepts, rootAccept{node: node, signal: signal})
		}
	}
	if len(accepts) == 0 {
		return
	}
	seen := map[*symbols.Symbol]bool{}
	for _, d := range deferred {
		if c.ctx.DownstreamOfFailure(d.Type) {
			continue
		}
		signal := c.symbolOf(d.Scope, d.Type)
		if signal == nil || seen[signal] {
			continue
		}
		seen[signal] = true
		var unmarked []*ast.Usage
		kept := false
		for _, a := range accepts {
			if a.signal != signal {
				continue
			}
			if lower.IsDeferredKeeper(c.resolver, scope, a.node) {
				kept = true
				break
			}
			unmarked = append(unmarked, a.node)
		}
		if kept {
			continue
		}
		for _, node := range unmarked {
			c.diags = append(c.diags, diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Span:     node.Span(),
				Message: fmt.Sprintf(
					"accept of deferred signal %s is not marked #MigrationMetadata::DeferredKeeper, so it is an ordinary accept, not the keeping loop; re-migrate the model or mark it",
					qnText(d.Type)),
				Code:   CodeDeferredKeeperUnmarked,
				Source: lintSource,
			})
		}
	}
}

// acceptedSignal is the signal the payload of the accept node, written in
// scope, is typed by; nil for any other action usage or an unresolved type.
func (c *deferredKeeperLint) acceptedSignal(scope *symbols.Scope, node *ast.Usage) *symbols.Symbol {
	payload := acceptPayload(node)
	if payload == nil {
		return nil
	}
	typeRef := typingTargetOf(payload)
	if typeRef == nil || c.ctx.DownstreamOfFailure(typeRef) {
		return nil
	}
	return c.symbolOf(scopeOwned(scope, node), typeRef)
}

// symbolOf resolves qn in scope, through any alias; nil without a resolution.
func (c *deferredKeeperLint) symbolOf(scope *symbols.Scope, qn *ast.QualifiedName) *symbols.Symbol {
	if scope == nil || qn == nil {
		return nil
	}
	sym, ok := c.resolver.ReadQualified(scope, qn).Symbol()
	if !ok || sym == nil {
		return nil
	}
	if target, ok := c.resolver.ResolveAliasTarget(sym); ok && target != nil {
		return target
	}
	return sym
}

// acceptPayload is the payload parameter of an accept node, nil for any other
// action usage.
func acceptPayload(node *ast.Usage) *ast.Usage {
	for _, member := range node.Members {
		if m, ok := memberNode(member).(*ast.Usage); ok && m.IsAccept {
			return m
		}
	}
	return nil
}

// typingTargetOf is the name the usage's first typing relationship states.
func typingTargetOf(usage *ast.Usage) *ast.QualifiedName {
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		if qn := ast.AsQualifiedName(rel.Target); qn != nil && len(qn.Parts) > 0 {
			return qn
		}
	}
	return nil
}

// memberNode is the declaration a membership wraps, or the node itself.
func memberNode(node ast.Node) ast.Node {
	if m, ok := node.(*ast.Membership); ok {
		return m.Member
	}
	return node
}

// scopeOwned is the scope decl owns under parent, or parent when it owns none.
func scopeOwned(parent *symbols.Scope, decl ast.Node) *symbols.Scope {
	if parent == nil {
		return nil
	}
	if child := parent.ChildFor(decl); child != nil {
		return child
	}
	return parent
}
