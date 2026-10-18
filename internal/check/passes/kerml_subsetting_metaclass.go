package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// KerMLSubsettingMetaclassPass checks that a feature subsets or redefines a
// Feature (KerML 8.3.3.3), in SysML sources as in KerML ones.
type KerMLSubsettingMetaclassPass struct{}

func (KerMLSubsettingMetaclassPass) Level() PassLevel { return LevelType }

func (KerMLSubsettingMetaclassPass) ElementScoped() { /* marker: per-element gating */ }

func (KerMLSubsettingMetaclassPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &kermlSubsettingMetaclassChecker{ctx: ctx}
	w := &kit.Walker{Ctx: ctx}
	w.Walk(rootScope, c.check)
	return c.diags
}

type kermlSubsettingMetaclassChecker struct {
	ctx   *Context
	diags []diag.Diagnostic
}

func (c *kermlSubsettingMetaclassChecker) check(sym *symbols.Symbol) {
	usage, ok := w8cUsageOf(sym)
	if !ok || !sym.IsFeature() {
		return
	}
	for _, rel := range usage.Relationships {
		if rel == nil || (rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines) ||
			rel.Target == nil || c.ctx.DownstreamOfFailure(rel.Target) {
			continue
		}
		// The reference form (`satisfy r;`) is an OwnedReferenceSubsetting the
		// referent-kind rule judges by what the usage must reference.
		if ast.IsReferenceSubsetting(usage, rel) {
			continue
		}
		target, ok := c.resolveTarget(sym, usage, rel)
		if !ok || target == nil {
			continue
		}
		if target.Kind == symbols.SymbolAlias {
			if resolved, ok := c.ctx.Resolver().ResolveAliasTarget(target); ok && resolved != nil {
				target = resolved
			}
		}
		if target.Kind != symbols.SymbolKerMLType && !target.Kind.IsDefinition() {
			continue
		}
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Target.Span(),
			Message:  fmt.Sprintf("%s target must be a feature, found %s", rel.Kind, target.Kind),
			Code:     codeSubsettingFeature,
			Source:   "type",
		})
	}
}

// resolveTarget resolves rel's target as the document walk does: a redefined
// feature is resolved past the redefining usage's borrowed name (KerML 8.3.3.3.6).
func (c *kermlSubsettingMetaclassChecker) resolveTarget(sym *symbols.Symbol, usage *ast.Usage, rel *ast.Relationship) (*symbols.Symbol, bool) {
	scope := kit.DeclarationScope(sym)
	if rel.Kind == ast.RelRedefines {
		return c.ctx.Resolver().ResolveRedefinitionTarget(scope, usage, rel.Target)
	}
	return c.ctx.Resolver().ResolveTarget(scope, rel.Target)
}
