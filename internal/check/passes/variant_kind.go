package passes

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// VariantKindPass checks that the kind of a variant usage is consistent with
// its owning variation (SysML v2 §7.6.7). A variant specializes the variation
// it is a variant of (§8.3.9.11 checkUsageVariationDefinitionSpecialization,
// checkUsageVariationUsageSpecialization), so the variation's types are the
// variant's too, and the kind's typing rule judges them like a declared type.
type VariantKindPass struct{}

func (VariantKindPass) Level() PassLevel { return LevelConstraint }

// ElementScoped: a variant gates on its own declaration and on what makes its
// owner a variation, not on faults elsewhere in the document.
func (VariantKindPass) ElementScoped() { /* marker: per-element gating */ }

func (VariantKindPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil || ctx.Kind == source.KindKerML {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &variantKindChecker{ctx: ctx, model: ctx.Model(), resolver: ctx.Resolver()}
	kit.WalkSymbols(ctx, rootScope, c.check)
	return c.diags
}

type variantKindChecker struct {
	ctx      *Context
	model    *semantics.Model
	resolver *resolve.Resolver
	diags    []diag.Diagnostic
}

// check judges one declared variant by the types its owning variation gives it.
// A bare `variant x;` restates a separately declared usage and declares no kind.
func (c *variantKindChecker) check(sym *symbols.Symbol) {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || !u.IsVariant || u.IsVariantReference() || c.ctx.DownstreamOfFailure(u) {
		return
	}
	variation := c.model.VariationPointOwning(sym)
	if variation == nil || variantOwnerUnsound(c.ctx, sym) {
		return
	}
	decl := declKind{
		useKind:      u.Kind,
		direction:    u.Direction,
		isReference:  u.IsReference,
		isEnd:        u.IsEnd,
		isIndividual: u.IsIndividual,
		portion:      u.Portion,
		keyword:      u.Keyword,
		hasType:      hasTypingRelationship(u.Relationships),
		span:         u.Span(),
		sym:          sym,
	}
	msg, _, ok := pilotTypingMessage(decl)
	// An `individual`/`snapshot`/`timeslice` variant is an occurrence whatever
	// its keyword, judged by the occurrence rule (§8.3.9.7 validateOccurrenceUsageType).
	if !ok && decl.isOccurrenceUsage() && !decl.conjugated {
		msg, ok = msgOccurrenceUsageType, true
	}
	if !ok {
		return
	}
	declared := c.declaredTypes(sym, u)
	for _, typ := range c.variationTypes(variation) {
		// A declared typing is the type checker's; an unresolved type constrains
		// nothing. A KerML type is judged by the metaclass its keyword names.
		if typ == nil || typ.Kind == symbols.SymbolUnknown || !isDefKind(typ.Kind) || containsSameElement(declared, typ) {
			continue
		}
		if usageTypeAllowed(c.model, c.resolver, decl, typ) {
			continue
		}
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     u.Span(),
			Message:  variantKindMessage(msg, strings.TrimSpace(decl.occurrenceModifier()+" "+decl.keyword), sym, variation, typ),
			Code:     "variant-kind",
			Source:   "constraint",
		})
		return
	}
}

// variationTypes are the types a variant takes from its owning variation: a
// variation definition itself, or the types of a variation usage (its declared
// ones, the ones it inherits, or the base its kind implies).
func (c *variantKindChecker) variationTypes(variation *symbols.Symbol) []*symbols.Symbol {
	if !variation.IsFeature() {
		return []*symbols.Symbol{variation}
	}
	return c.model.FeatureTypeSet(variation)
}

// declaredTypes resolves the typings the variant declares itself.
func (c *variantKindChecker) declaredTypes(sym *symbols.Symbol, u *ast.Usage) []*symbols.Symbol {
	var out []*symbols.Symbol
	scope := kit.ReferenceScope(sym)
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		if target, ok := c.resolver.ResolveTarget(scope, rel.Target); ok && target != nil {
			out = append(out, target)
		}
	}
	return out
}

func containsSameElement(syms []*symbols.Symbol, sym *symbols.Symbol) bool {
	for _, s := range syms {
		if symbols.SameElement(s, sym) {
			return true
		}
	}
	return false
}

// variantKindMessage names the variant, its owning variation and the type the
// variation gives it, after the kind rule's own message.
func variantKindMessage(rule, keyword string, variant, variation, typ *symbols.Symbol) string {
	if keyword == "" {
		keyword = variant.Notation()
	}
	// Notation spells the variation's kind as written, `variation part def`.
	head := fmt.Sprintf("%s variant %s %s specializes its owning %s %s", rule, keyword, w8dSymbolName(variant), variation.Notation(), w8dSymbolName(variation))
	if symbols.SameElement(variation, typ) {
		return head + "."
	}
	return fmt.Sprintf("%s, which is typed by %s %s.", head, typ.Notation(), w8dSymbolName(typ))
}
