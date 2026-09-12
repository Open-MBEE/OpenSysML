package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// VariantOwnerPass reports a `variant` whose owner is not a variation: it offers
// no choice to anything (SysML v2 §7.20 VariantMembership; pilot SysMLValidator
// validateVariationMembershipOwningNamespace). The grammar admits the member in any
// definition body (SysML.xtext DefinitionBodyItem), so the rule is semantic.
type VariantOwnerPass struct{}

func (VariantOwnerPass) Level() PassLevel { return LevelConstraint }

// ElementScoped: a variant gates on its own declaration and on what makes its
// owner a variation, not on faults elsewhere in the document.
func (VariantOwnerPass) ElementScoped() {}

func (VariantOwnerPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	var diags []Diagnostic
	model := ctx.Model()
	w8dWalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		if d, ok := variantOwnerDiagnostic(ctx, model, sym); ok {
			diags = append(diags, d)
		}
	})
	return diags
}

// variantOwnerDiagnostic judges one declared variant. An enumeration body admits
// no `variant` keyword: its values are variants already.
func variantOwnerDiagnostic(ctx *Context, model *semantics.Model, sym *symbols.Symbol) (Diagnostic, bool) {
	if !semantics.DeclaresVariant(sym) || ctx.DownstreamOfFailure(sym.Decl) {
		return Diagnostic{}, false
	}
	msg := msgVariantOutsideVariation
	if owner := semantics.EnumerationDefinitionOwning(sym); owner != nil {
		msg = fmt.Sprintf("`variant` is not written in enumeration definition %s: every enumerated value of an enumeration definition is already a variant; drop the keyword", w8dSymbolName(owner))
	} else if variantOwnerUnsound(ctx, sym) || model.VariationPointOwning(sym) != nil {
		return Diagnostic{}, false
	}
	return Diagnostic{
		Severity: SeverityError,
		Span:     sym.Decl.Span(),
		Message:  msg,
		Code:     "variant-outside-variation",
		Source:   "constraint",
	}, true
}

// variantOwnerUnsound reports whether a lower tier faulted a specialization the
// owner's standing as a variation point may rest on.
func variantOwnerUnsound(ctx *Context, sym *symbols.Symbol) bool {
	if sym.OwnerScope == nil {
		return false
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return false
	}
	for _, rel := range semantics.RelationshipsOf(owner) {
		if rel != nil && rel.Target != nil && ctx.DownstreamOfFailure(rel.Target) {
			return true
		}
	}
	return false
}
