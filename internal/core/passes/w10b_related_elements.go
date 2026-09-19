package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// msgRelatedElements is the reference's message for an association or connector
// that relates fewer than two elements (pilot KerMLValidator checkAssociation,
// validateAssociationRelatedTypes; checkConnector, validateConnectorRelatedFeatures).
const msgRelatedElements = "Must have at least two related elements"

// msgBindingBinary is the reference's message for a binding connector whose ends
// reference other than two features (pilot validateBindingConnectorIsBinary).
const msgBindingBinary = "Binding connector must be binary"

// W10BRelatedElementsPass reports a concrete association with fewer than two
// ends, or a concrete connector whose ends reference fewer than two features: a
// link needs two participants (KerML 1.1 §8.3.3.5, §8.3.4.7). An abstract
// declaration is exempt, as it is in the reference — its ends are the
// specialization's to supply. A binding connector, abstract or not, must bind
// exactly two features (§8.3.4.7.2 validateBindingConnectorIsBinary).
type W10BRelatedElementsPass struct{}

func (W10BRelatedElementsPass) Level() PassLevel { return LevelConstraint }

func (W10BRelatedElementsPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	model := ctx.Model()
	isKerML := ctx.Kind == source.KindKerML
	var diags []diag.Diagnostic
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		if u, ok := sym.Decl.(*ast.Usage); ok && u.Kind == ast.UsageBinding && model.RelatedFeatureCount(sym) != 2 {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.SeverityError,
				Span:     u.Span(),
				Message:  msgBindingBinary,
				Code:     "binding-binary",
				Source:   "constraint",
			})
		}
		switch w10bClassify(sym, isKerML) {
		case w10bAssociation:
			if w10bEndCount(model, sym, make(map[*symbols.Symbol]bool)) >= 2 {
				return
			}
		case w10bConnector:
			if model.RelatedFeatureCount(sym) >= 2 {
				return
			}
		default:
			return
		}
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     sym.Decl.Span(),
			Message:  msgRelatedElements,
			Code:     "related-elements",
			Source:   "constraint",
		})
	})
	return diags
}

// w10bEndCount counts the ends of sym, its own or those it inherits from what
// it types by, subsets or redefines. A variation supplies its ends through the
// variant selected for it, so it counts as related.
func w10bEndCount(model *semantics.Model, sym *symbols.Symbol, visited map[*symbols.Symbol]bool) int {
	if sym == nil || visited[sym] {
		return 0
	}
	visited[sym] = true
	if semantics.DeclaresVariation(sym) {
		return 2
	}
	count := model.ConnectorEndCount(sym)
	for _, target := range model.DirectSupertypes(sym) {
		if count >= 2 {
			break
		}
		if inherited := w10bEndCount(model, target, visited); inherited > count {
			count = inherited
		}
	}
	return count
}

// w10bRelationKind tells an association, whose ends are its related types,
// from a connector, whose related features are what its ends reference.
type w10bRelationKind int

const (
	w10bUnrelated w10bRelationKind = iota
	w10bAssociation
	w10bConnector
)

// w10bClassify classifies sym for this rule. Abstract declarations and
// variations are exempt, as is a SysML flow with no ends: a message, which the
// reference makes abstract.
func w10bClassify(sym *symbols.Symbol, isKerML bool) w10bRelationKind {
	if sym == nil || semantics.DeclaresVariation(sym) {
		return w10bUnrelated
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		if d.IsAbstract {
			return w10bUnrelated
		}
		switch d.Kind {
		case ast.DefConnection, ast.DefInterface, ast.DefAllocation, ast.DefFlow, ast.DefAssoc:
			return w10bAssociation
		}
	case *ast.Usage:
		if d.IsAbstract {
			return w10bUnrelated
		}
		switch d.Kind {
		case ast.UsageAssoc, ast.UsageInteraction:
			return w10bAssociation
		case ast.UsageFlow:
			if !isKerML && !w10bFlowStatesEnd(d) && !w10bDeclaresEnd(d) {
				return w10bUnrelated
			}
			return w10bConnector
		case ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation,
			ast.UsageConnector, ast.UsageBinding, ast.UsageSuccession:
			return w10bConnector
		}
	}
	return w10bUnrelated
}

// w10bFlowStatesEnd reports whether a flow names a `from`/`to` end; `of T`
// alone states only the payload.
func w10bFlowStatesEnd(usage *ast.Usage) bool {
	return usage.FlowEnds != nil && (usage.FlowEnds.From != nil || usage.FlowEnds.To != nil)
}

// w10bDeclaresEnd reports whether usage owns an `end` feature in its body.
func w10bDeclaresEnd(usage *ast.Usage) bool {
	for _, member := range usage.Members {
		if wrapper, ok := member.(*ast.Membership); ok {
			member = wrapper.Member
		}
		if end, ok := member.(*ast.Usage); ok && end.IsEnd {
			return true
		}
	}
	return false
}
