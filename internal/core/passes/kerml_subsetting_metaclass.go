package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// KerMLSubsettingMetaclassPass checks that a feature subsets another Feature.
type KerMLSubsettingMetaclassPass struct{}

func (KerMLSubsettingMetaclassPass) Level() PassLevel { return LevelType }

func (KerMLSubsettingMetaclassPass) ElementScoped() { /* marker: per-element gating */ }

func (KerMLSubsettingMetaclassPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil || ctx.Kind != source.KindKerML {
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
		if rel == nil || rel.Kind != ast.RelSubsets || rel.Target == nil ||
			c.ctx.DownstreamOfFailure(rel.Target) {
			continue
		}
		target, ok := c.ctx.Resolver().ResolveTarget(kit.DeclarationScope(sym), rel.Target)
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
			Code:     "type",
			Source:   "type",
		})
	}
}
