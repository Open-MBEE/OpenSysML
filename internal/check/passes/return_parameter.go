package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// msgReturnParameterOwner reports a `return` parameter owned by a type that is
// no function or expression (KerML validateReturnParameterMembershipOwningType).
const msgReturnParameterOwner = "Return parameter membership not allowed: only a function or expression (calculation, constraint) declares a result; write `out` for an output"

// checkReturnParameterOwner reports a result parameter whose owning type is no
// function or expression.
func (cc *constraintChecker) checkReturnParameterOwner(sym *symbols.Symbol) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !usage.IsResult || semantics.ResultParameterOwnerValid(sym) {
		return
	}
	cc.diags = append(cc.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     usage.Span(),
		Message:  msgReturnParameterOwner,
		Code:     "return-parameter-owner",
		Source:   "constraint",
	})
}
