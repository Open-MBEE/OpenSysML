package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot KerMLValidator (2026-05) checkMetadataBodyFeature, constraint
// INVALID_METADATA_FEATURE_BODY.
const msgMetadataBodyFeature = "Must redefine an owning-type feature"

// W8DMetadataUsagePass checks that every feature in a `metadata m : A { … }` usage body
// redefines one of A's own and binds a model-level evaluable value (KerML §7.5);
// the type itself is MetadataTypePass's. It runs at the tier MetadataAnnotationPass
// reports the same rules for the `@A { … }` spelling.
type W8DMetadataUsagePass struct{}

func (W8DMetadataUsagePass) Level() PassLevel { return LevelType }

func (W8DMetadataUsagePass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	mc := &w8dMetadataChecker{resolver: ctx.Resolver(), model: ctx.Model()}
	if mc.resolver == nil || mc.model == nil {
		return nil
	}
	kit.WalkSymbols(ctx, rootScope, mc.check)
	return mc.diags
}

type w8dMetadataChecker struct {
	resolver *resolve.Resolver
	model    *semantics.Model
	diags    []diag.Diagnostic
}

func (mc *w8dMetadataChecker) check(sym *symbols.Symbol) {
	if u, ok := sym.Decl.(*ast.Usage); ok && u.Kind == ast.UsageMetadata {
		mc.checkMetadataUsage(sym, u)
	}
}

// checkMetadataUsage checks `metadata m : A { … }`, whose annotated metadata
// definition is what the usage is typed by.
func (mc *w8dMetadataChecker) checkMetadataUsage(sym *symbols.Symbol, u *ast.Usage) {
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		mc.checkBody(sym, qn, u.Members)
		return
	}
}

func (mc *w8dMetadataChecker) checkBody(sym *symbols.Symbol, typeRef *ast.QualifiedName, body []ast.Node) {
	if sym.OwnerScope == nil || typeRef == nil {
		return
	}
	typ, ok := mc.resolver.ResolveQualified(sym.OwnerScope, typeRef)
	if !ok || typ == nil {
		return
	}
	if resolved, aliasOK := mc.resolver.ResolveAliasTarget(typ); aliasOK {
		typ = resolved
	} else {
		return
	}
	// A type that is no concrete metaclass is MetadataTypePass's report; its
	// body would only repeat it.
	if !semantics.IsMetadataType(typ) || symbols.IsAbstract(typ) {
		return
	}
	for _, node := range mc.model.MetadataBodyViolationsOf(typ, sym.Scope, body) {
		mc.diags = append(mc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     node.Span(),
			Message:  msgMetadataBodyFeature,
			Code:     "metadata-body-feature",
			Source:   "constraint",
		})
	}
	for _, value := range mc.model.MetadataBodyInevaluableValuesOf(typ, sym.Scope, body) {
		mc.diags = append(mc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     metadataValueSpan(body, value),
			Message:  msgFilterNotEvaluable,
			Code:     "metadata-value-not-evaluable",
			Source:   "constraint",
		})
	}
}
