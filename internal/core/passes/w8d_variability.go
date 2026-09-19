package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Pilot SysMLValidator (2026-07) checkDefinition/checkUsage, constraints
// validateDefinitionVariationMembership/validateUsageVariationMembership and
// validateDefinitionVariationSpecialization/validateUsageVariationSpecialization.
const (
	msgVariationMemberNotVariant = "An owned usage of a variation must be a variant."
	msgVariationSpecialization   = "A variation must not specialize another variation."
	msgVariantOutsideVariation   = "A variant must be an owned member of a variation."
	msgEnumerationIsVariation    = "every enumeration definition is a variation whose enumerated values are its variants"
)

// W8DVariabilityPass checks that every usage a variation owns is a variant and
// that a variation does not specialize another variation (SysML v2 §7.20). An
// enumeration definition is a variation without saying so (SysML v2 §7.6.4).
type W8DVariabilityPass struct{}

func (W8DVariabilityPass) Level() PassLevel { return LevelConstraint }

func (W8DVariabilityPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	vc := &w8dVariabilityChecker{resolver: ctx.Resolver()}
	kit.WalkSymbols(ctx, rootScope, vc.check)
	return vc.diags
}

type w8dVariabilityChecker struct {
	resolver *resolve.Resolver
	diags    []diag.Diagnostic
}

func (vc *w8dVariabilityChecker) check(sym *symbols.Symbol) {
	if !semantics.IsVariation(sym) {
		return
	}
	vc.checkMembers(sym)
	vc.checkSpecializations(sym)
}

// checkMembers reports every owned usage of a variation that is not a variant;
// an objective, a metadata usage (an annotation), and an enumerated value are exempt.
func (vc *w8dVariabilityChecker) checkMembers(sym *symbols.Symbol) {
	_, isUsage := sym.Decl.(*ast.Usage)
	isEnum := sym.Kind == symbols.SymbolEnumerationDef
	for _, member := range ast.DeclMembers(sym.Decl) {
		node := unwrapType(member)
		if _, ok := node.(*ast.SubjectMember); ok {
			vc.reportMember(sym, member.Span(), "")
			continue
		}
		u, ok := node.(*ast.Usage)
		if !ok || u.IsVariant || u.Kind == ast.UsageMetadata {
			continue
		}
		if isUsage && u.Kind == ast.UsageObjective {
			continue
		}
		if isEnum && u.Kind == ast.UsageEnumeration {
			continue
		}
		vc.reportMember(sym, member.Span(), u.Ident.Name)
	}
}

func (vc *w8dVariabilityChecker) reportMember(owner *symbols.Symbol, span source.Span, name string) {
	msg := msgVariationMemberNotVariant
	if owner.Kind == symbols.SymbolEnumerationDef {
		member := "this member"
		if name != "" {
			member = "`" + name + "`"
		}
		msg = fmt.Sprintf("%s cannot own %s: %s; declare %s as an enumerated value or in an attribute definition that %s specializes",
			w8dVariationNoun(owner), member, msgEnumerationIsVariation, member, w8dSymbolName(owner))
	}
	vc.diags = append(vc.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     span,
		Message:  msg,
		Code:     "variation-member-not-variant",
		Source:   "constraint",
	})
}

// checkSpecializations reports a variation whose declared general is itself a
// variation; a typing counts, since FeatureTyping is a Specialization.
func (vc *w8dVariabilityChecker) checkSpecializations(sym *symbols.Symbol) {
	scope := sym.OwnerScope
	if sym.Scope != nil {
		scope = sym.Scope
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || !w8dSpecializationKind(rel.Kind) {
			continue
		}
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, ok := targetNode.(*ast.QualifiedName)
		if !ok {
			continue
		}
		general, ok := vc.resolver.ResolveQualified(scope, qn)
		if !ok || general == sym || !semantics.IsVariation(general) {
			continue
		}
		vc.diags = append(vc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Target.Span(),
			Message:  w8dSpecializationMessage(sym, general, rel.Kind),
			Code:     "variation-specialization",
			Source:   "constraint",
		})
	}
}

// w8dSpecializationMessage keeps the reference wording between two declared
// variations and spells out the implicit one when an enumeration is involved.
func w8dSpecializationMessage(sym, general *symbols.Symbol, kind ast.RelationshipKind) string {
	if sym.Kind != symbols.SymbolEnumerationDef && general.Kind != symbols.SymbolEnumerationDef {
		return msgVariationSpecialization
	}
	verb := "specialize"
	if kind == ast.RelTyping {
		verb = "be typed by"
	}
	msg := fmt.Sprintf("%s must not %s %s: %s, and a variation must not specialize another variation",
		w8dVariationNoun(sym), verb, w8dVariationNoun(general), msgEnumerationIsVariation)
	if sym.Kind == symbols.SymbolEnumerationDef {
		msg += fmt.Sprintf("; declare the enumerated values of %s directly", w8dSymbolName(sym))
	}
	return msg
}

func w8dVariationNoun(sym *symbols.Symbol) string {
	if sym.Kind == symbols.SymbolEnumerationDef {
		return "enumeration definition " + w8dSymbolName(sym)
	}
	return "variation " + w8dSymbolName(sym)
}

func w8dSymbolName(sym *symbols.Symbol) string {
	if sym.Name == "" {
		return "(unnamed)"
	}
	return "`" + sym.Name + "`"
}

// w8dSpecializationKind reports whether kind is an owned specialization of the
// declaration: a typing is one (FeatureTyping specializes Specialization).
func w8dSpecializationKind(kind ast.RelationshipKind) bool {
	switch kind {
	case ast.RelTyping, ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines:
		return true
	}
	return false
}
