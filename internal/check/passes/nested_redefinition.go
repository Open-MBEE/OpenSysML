package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// CodeRedefinitionThroughReference marks a chain redefinition walking through
// a feature that owns no object — a reference, subject or port usage — so the
// redefinition has nothing below it to apply on.
const CodeRedefinitionThroughReference = "redefinition-through-reference"

// NestedRedefinitionPass reports a chain redefinition (`:>> mid.leaf.value`)
// whose path crosses a reference usage: the reference owns no object below it
// for the redefinition to apply on.
type NestedRedefinitionPass struct{}

// Level reports the constraint level: the pass reads resolved symbols.
func (NestedRedefinitionPass) Level() PassLevel { return LevelConstraint }

// Run reports each nested redefinition of the document crossing a reference.
func (NestedRedefinitionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	p := &nestedRedefinitionChecker{
		model: ctx.Model(),
		seen:  make(map[*symbols.Symbol]bool),
	}
	p.walk(rootScope)
	return p.diags
}

type nestedRedefinitionChecker struct {
	model *semantics.Model
	seen  map[*symbols.Symbol]bool
	diags []diag.Diagnostic
}

func (p *nestedRedefinitionChecker) walk(scope *symbols.Scope) {
	if scope == nil {
		return
	}
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if sym == nil || p.seen[sym] {
			return true
		}
		p.seen[sym] = true
		p.check(sym)
		p.walk(sym.Scope)
		return true
	})
}

// check reports each nested redefinition sym declares whose chain crosses a
// reference, subject or port feature.
func (p *nestedRedefinitionChecker) check(sym *symbols.Symbol) {
	for _, nr := range p.model.NestedRedefinitionsOf(sym) {
		p.checkChainSegments(sym, nr)
	}
}

// checkChainSegments reports each non-final segment of the chain that resolves
// to a feature owning no object: nothing below it can be redefined at runtime.
func (p *nestedRedefinitionChecker) checkChainSegments(owner *symbols.Symbol, nr semantics.NestedRedefinition) {
	rel := redefinesChain(nr.Feature)
	if rel == nil {
		return
	}
	for i, node := range chainPrefixNodes(rel.Target) {
		resolved := p.model.RelationshipTarget(nr.Feature, &ast.Relationship{Kind: ast.RelRedefines, Target: node})
		if resolved == nil {
			continue
		}
		if !semantics.IsReferenceUsage(resolved) && !semantics.IsSubjectUsage(resolved) &&
			!p.model.ReferentialParameter(resolved) && resolved.Kind != symbols.SymbolPortUsage {
			continue
		}
		p.diags = append(p.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Target.Span(),
			Message: fmt.Sprintf(
				"nested redefinition through reference %s has no owned object to redefine on",
				nr.Path[i]),
			Code:   CodeRedefinitionThroughReference,
			Source: "constraint",
		})
		return
	}
}

// redefinesChain returns the chain redefinition relationship feature declares.
func redefinesChain(feature *symbols.Symbol) *ast.Relationship {
	for _, rel := range semantics.RelationshipsOf(feature) {
		if rel != nil && rel.Kind == ast.RelRedefines {
			if _, ok := rel.Target.(*ast.FeatureChainExpr); ok {
				return rel
			}
		}
	}
	return nil
}

// chainPrefixNodes returns the nodes of a chain target that resolve its
// non-final segments, outermost first: each operand, and each nested chain of
// one as its own last segment.
func chainPrefixNodes(node ast.Node) []ast.Node {
	chain, ok := node.(*ast.FeatureChainExpr)
	if !ok {
		return nil
	}
	var out []ast.Node
	if inner, ok := chain.Operand.(*ast.FeatureChainExpr); ok {
		out = append(chainPrefixNodes(inner), inner)
	} else {
		out = append(out, chain.Operand)
	}
	return out
}
