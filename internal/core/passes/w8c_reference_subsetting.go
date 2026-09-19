package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// KerMLValidator's validateFeatureOwnedReferenceSubsetting message.
const msgReferenceSubsettingAtMostOne = "At most one reference subsetting is allowed"

// ReferenceSubsettingPass checks that a feature owns at most one reference
// subsetting (`references`/`::>`), KerML 8.3.3.1.5. Every reference subsetting
// after the first is reported, as the pilot does.
type ReferenceSubsettingPass struct{}

func (ReferenceSubsettingPass) Level() PassLevel { return LevelConstraint }

func (ReferenceSubsettingPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	var diags []diag.Diagnostic
	w := &kit.Walker{Ctx: ctx}
	w.Walk(rootScope, func(sym *symbols.Symbol) {
		var refs []*ast.Relationship
		for _, rel := range semantics.RelationshipsOf(sym) {
			if rel != nil && rel.Kind == ast.RelReferences && rel.Target != nil {
				refs = append(refs, rel)
			}
		}
		for _, rel := range refs[min(1, len(refs)):] {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.SeverityError,
				Span:     rel.Target.Span(),
				Message:  msgReferenceSubsettingAtMostOne,
				Code:     "reference-subsetting-at-most-one",
				Source:   "constraint",
			})
		}
	})
	return diags
}
