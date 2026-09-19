package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const msgEnumerationAttributeTypes = "An enumeration attribute cannot have more than one type."

// Pilot SysMLValidator (2026-05) checkOneType: these usage kinds are typed by
// exactly one definition, so a second declared type is an error. Messages are
// the reference's, per kind.
var oneTypeUsageMessages = map[ast.UsageKind]string{
	ast.UsageCalc:             "A calculation must be typed by one calculation definition.",
	ast.UsageConstraint:       "A constraint must be typed by one constraint definition.",
	ast.UsageRequirement:      "A requirement must be typed by one requirement definition.",
	ast.UsageCase:             "A case must be typed by one case definition.",
	ast.UsageAnalysisCase:     "An analysis case must be typed by one analysis case definition.",
	ast.UsageVerificationCase: "A verification case must be typed by one verification case definition.",
	ast.UsageUseCase:          "A use case must be typed by one use case definition.",
	ast.UsageEnumeration:      "An enumeration must be typed by one enumeration definition.",
	ast.UsageRendering:        "A rendering must be typed by one rendering definition.",
	ast.UsageViewpoint:        "A viewpoint must be typed by one viewpoint definition.",
	ast.UsageView:             "A view must be typed by one view definition.",
	ast.UsageMetadata:         "A metadata usage must be typed by one metadata definition.",
}

// checkOneType reports a usage that declares more than one type where the
// reference admits one. A usage declaring none is silent: the reference counts
// the type its library base supplies, so there is exactly one.
func (tc *typeChecker) checkOneType(scope *symbols.Scope, d featureDecl) {
	typings := 0
	for _, rel := range d.relationships {
		if rel != nil && rel.Kind == ast.RelTyping {
			typings++
		}
	}
	if typings <= 1 && !tc.enumeratedValueTypeDiffers(scope, d) {
		return
	}
	msg, ok := oneTypeUsageMessages[d.kind]
	// An `attribute` typed by an enumeration takes no other type (SysMLValidator
	// checkAttributeUsage); a bare or `ref` usage is a reference usage, not checked.
	if !ok && d.keyword == "attribute" && tc.typesAnEnumeration(scope, d.relationships) {
		msg, ok = msgEnumerationAttributeTypes, true
	}
	if !ok {
		return
	}
	tc.appendUnique(diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     d.span,
		Message:  msg,
		Code:     "one-type",
		Source:   "type",
	})
}

// enumeratedValueTypeDiffers reports an enumerated value typed — by declaration
// or by a typing value — outside its enumeration and that enumeration's generals.
func (tc *typeChecker) enumeratedValueTypeDiffers(scope *symbols.Scope, d featureDecl) bool {
	u, ok := d.node.(*ast.Usage)
	if !ok || u.Kind != ast.UsageEnumeration || tc.owningEnumeration(scope) == nil {
		return false
	}
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		if typ := tc.expr.resolveTarget(scope, rel.Target); typ != nil && !tc.owningEnumerationConformsTo(scope, typ) {
			return true
		}
	}
	for _, typ := range tc.enumeratedValueTypes(scope, u) {
		if !tc.owningEnumerationConformsTo(scope, typ) {
			return true
		}
	}
	return false
}

// enumeratedValueTypes are the types a non-default value gives an enumerated value
// declaring no generalization (KerML checkFeatureValuationSpecialization): the
// result its syntax declares, else those of the feature or function it names.
func (tc *typeChecker) enumeratedValueTypes(scope *symbols.Scope, u *ast.Usage) []*symbols.Symbol {
	if u.Value == nil || u.ValueIsDefault || u.Direction != ast.DirNone {
		return nil
	}
	for _, rel := range u.Relationships {
		if rel != nil && semantics.GeneralizationKind(rel.Kind) {
			return nil
		}
	}
	if types := tc.expr.model.ExprResultTypes(scope, u.Value); len(types) > 0 {
		return types
	}
	if typ := tc.expr.valueTypeSymbol(scope, u.Value); typ != nil {
		return []*symbols.Symbol{typ}
	}
	if typ := tc.expr.invocationResultTypeSymbol(scope, u.Value); typ != nil {
		return []*symbols.Symbol{typ}
	}
	return nil
}

// owningEnumeration is the enumeration definition whose body scope is, or nil.
func (tc *typeChecker) owningEnumeration(scope *symbols.Scope) *symbols.Symbol {
	if scope == nil || tc.expr.model == nil {
		return nil
	}
	if enum := scope.Owner(); enum != nil && enum.Kind == symbols.SymbolEnumerationDef {
		return enum
	}
	return nil
}

// owningEnumerationConformsTo reports a type an enumerated value in scope takes
// redundantly: its enumeration or a general of it (pilot removeRedundantTypes).
func (tc *typeChecker) owningEnumerationConformsTo(scope *symbols.Scope, typ *symbols.Symbol) bool {
	enum := tc.owningEnumeration(scope)
	return enum != nil && tc.expr.model.Conforms(enum, typ)
}

func (tc *typeChecker) typesAnEnumeration(scope *symbols.Scope, rels []*ast.Relationship) bool {
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		sym, ok := tc.resolver.ResolveQualified(scope, qn)
		if !ok || sym == nil {
			continue
		}
		if sym.Kind == symbols.SymbolAlias {
			if resolved, ok := tc.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
				sym = resolved
			}
		}
		if sym.Kind == symbols.SymbolEnumerationDef {
			return true
		}
	}
	return false
}
