package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// KerMLValidator's validateFunctionResultExpressionMembership and
// validateExpressionResultExpressionMembership message.
const msgResultExpressionAtMostOne = "Only one (owned or inherited) result expression is allowed"

// ResultExpressionPass checks that a function or expression has at most one
// result expression, owned or inherited (KerML 8.3.4.6 Expression::result,
// 8.3.4.8 Function::result). A calculation body states at most one; a type
// that inherits a result expression cannot state its own — a specialization or
// redefinition keeps the inherited body or adds constraints beside it — and one
// that inherits two along different specializations is invalid on its own.
type ResultExpressionPass struct{}

func (ResultExpressionPass) Level() PassLevel { return LevelConstraint }

// ElementScoped: each type gates on the head naming its supertypes.
func (ResultExpressionPass) ElementScoped() { /* marker: per-element gating */ }

func (ResultExpressionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	model := ctx.Model()
	var diags []diag.Diagnostic
	w := &kit.Walker{Ctx: ctx}
	w.Walk(rootScope, func(sym *symbols.Symbol) {
		if !semantics.FunctionLike(sym) {
			return
		}
		if head, typed := w8cOwnerHead(sym.Decl); typed && ctx.DownstreamSpan(head) {
			return
		}
		conflict := model.ResultExpressionConflict(sym)
		if conflict == nil {
			return
		}
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     conflict.Node.Span(),
			Message:  msgResultExpressionAtMostOne,
			Code:     "result-expression-at-most-one",
			Source:   "constraint",
		})
	})
	return diags
}
