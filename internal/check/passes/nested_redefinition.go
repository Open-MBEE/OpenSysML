package passes

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// CodeNonstandardSemantics marks semantics OpenSysML applies beyond what the
// pinned SysML v2 pilot does: the notation parses there, but the pilot does not
// act on it the way OpenSysML does.
const CodeNonstandardSemantics = "nonstandard-semantics"

// NestedRedefinitionPass reports the nested redefinition extension: a chain
// redefinition (`:>> mid.leaf.value`) written on a usage applies below the
// member it names in OpenSysML, while the pinned pilot accepts the notation but
// applies no redefinition below the first segment. It also reports a chain
// walking through a reference usage, which has no owned object to redefine on.
type NestedRedefinitionPass struct{}

// Level reports the constraint level: the pass reads resolved symbols.
func (NestedRedefinitionPass) Level() PassLevel { return LevelConstraint }

// Run reports each nested redefinition of the document.
func (NestedRedefinitionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	p := &nestedRedefinitionChecker{
		model:    ctx.Model(),
		severity: notationSeverity(ctx.Options.Conformance),
		seen:     make(map[*symbols.Symbol]bool),
	}
	p.walk(rootScope)
	return p.diags
}

type nestedRedefinitionChecker struct {
	model    *semantics.Model
	severity diag.Severity
	seen     map[*symbols.Symbol]bool
	diags    []diag.Diagnostic
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

// check reports the advisory for each nested redefinition sym declares, and the
// error for one whose chain crosses a reference, subject or port feature.
func (p *nestedRedefinitionChecker) check(sym *symbols.Symbol) {
	for _, nr := range p.model.NestedRedefinitionsOf(sym) {
		span := nr.Feature.Decl.Span()
		p.diags = append(p.diags, diag.Diagnostic{
			Severity: p.severity,
			Span:     span,
			Message: fmt.Sprintf(
				"nested redefinition of %s is an OpenSysML extension: the pinned SysML v2 pilot accepts the notation "+
					"but does not apply the redefinition below %s; write it as nested redefining usages for a pilot-portable model",
				strings.Join(nr.Path, "."), nr.Path[0]),
			Code:   CodeNonstandardSemantics,
			Source: "constraint",
		})
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
		if !semantics.IsReferenceUsage(resolved) && !semantics.IsSubjectUsage(resolved) && resolved.Kind != symbols.SymbolPortUsage {
			continue
		}
		p.diags = append(p.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Target.Span(),
			Message: fmt.Sprintf(
				"nested redefinition through reference %s has no owned object to redefine on",
				nr.Path[i]),
			Code:   CodeNonstandardSemantics,
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
