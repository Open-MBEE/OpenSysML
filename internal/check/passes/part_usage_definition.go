package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// msgPartUsagePartDefinition is the reference validator's message for
// validatePartUsagePartDefinition; the pilot declares the constant but its
// checkPartUsage is commented out, so it never reports it.
const msgPartUsagePartDefinition = "A part must be typed by at least one part definition."

// PartUsageDefinitionPass checks that a part usage is typed by at least one
// part definition (SysML v2 §8.3.11 validatePartUsagePartDefinition:
// partDefinition->notEmpty()).
type PartUsageDefinitionPass struct{}

func (PartUsageDefinitionPass) Level() PassLevel { return LevelConstraint }

func (PartUsageDefinitionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil || ctx.Kind == source.KindKerML {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &partUsageDefinitionChecker{model: ctx.Model(), index: ctx.Index}
	kit.WalkSymbols(ctx, rootScope, c.check)
	return c.diags
}

type partUsageDefinitionChecker struct {
	model *semantics.Model
	index *symbols.Index
	diags []diag.Diagnostic
}

func (c *partUsageDefinitionChecker) check(sym *symbols.Symbol) {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || u.Kind != ast.UsagePart || c.index.Library(sym) {
		return
	}
	// A bare `variant x;` is a VariantReference, not a PartUsage declaration;
	// the parser's part kind is a placeholder, so the rule does not read it.
	if u.IsVariantReference() {
		return
	}
	types := c.model.FeatureTypeSet(sym)
	if len(types) == 0 {
		return
	}
	for _, typ := range types {
		if typ != nil && typ.Kind == symbols.SymbolPartDef {
			return
		}
	}
	for _, typ := range types {
		// An unresolved or KerML type constrains nothing; a non-definition
		// type cannot satisfy partDefinition->notEmpty().
		if typ == nil || typ.Kind == symbols.SymbolUnknown || typ.Kind == symbols.SymbolKerMLType || !isDefKind(typ.Kind) {
			return
		}
	}
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     u.Span(),
		Message:  msgPartUsagePartDefinition,
		Code:     "part-usage-part-definition",
		Source:   "constraint",
	})
}
