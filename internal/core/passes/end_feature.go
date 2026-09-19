package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// KerMLValidator's validateFeatureEndNoDirection message.
const msgEndFeatureDirection = "End feature cannot have direction"

// KerMLValidator's validateFeatureEndNotDerivedAbstractCompositeOrPortion message.
const msgEndFeatureKind = "End feature cannot be derived, abstract, composite or portion"

// EndFeaturePass checks end features (KerML 8.3.3.1.5): an end feature has no
// direction and is not derived, abstract, composite or portion.
type EndFeaturePass struct{}

func (EndFeaturePass) Level() PassLevel { return LevelConstraint }

// ElementScoped: each end feature gates on its own head.
func (EndFeaturePass) ElementScoped() { /* marker: per-element gating */ }

func (EndFeaturePass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	var diags []diag.Diagnostic
	report := func(u *ast.Usage, message, code string) {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     w8cUsageHead(u),
			Message:  message,
			Code:     code,
			Source:   "constraint",
		})
	}
	(&kit.Walker{Ctx: ctx}).Walk(rootScope, func(sym *symbols.Symbol) {
		u, ok := sym.Decl.(*ast.Usage)
		if !ok || !u.IsEnd || ctx.DownstreamSpan(w8cUsageHead(u)) {
			return
		}
		if u.Direction != ast.DirNone {
			report(u, msgEndFeatureDirection, "end-feature-direction")
		}
		// A `variation` usage is abstract.
		if u.IsDerived || u.IsAbstract || u.IsVariation || u.IsComposite || u.IsPortion {
			report(u, msgEndFeatureKind, "end-feature-kind")
		}
	})
	return diags
}
