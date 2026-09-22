package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// codeUndefinedOperator marks a use of the unary `~`, the operator KerML 1.0
// §8.2.5.8.1 maps to the abstract `DataFunctions::'~'` and leaves undefined.
const codeUndefinedOperator = "undefined-operator"

const msgUndefinedOperator = "operator '~' invokes DataFunctions::'~', which the Kernel Function Library declares abstract and leaves undefined; no library the runtime applies defines it, so the expression has no value"

// UndefinedOperatorPass warns on every written `~x`, as §8.2.5.8.1 asks of a
// tool when no domain-specific definition of `DataFunctions::'~'` is available.
type UndefinedOperatorPass struct{}

// Level reports the syntax level: the written operator is all it reads, so a
// later-tier failure never hides the warning.
func (UndefinedOperatorPass) Level() PassLevel { return LevelSyntax }

// Run walks the parsed tree and warns at each `~` operator expression. It stays
// a warning in every conformance mode: the specification asks for a warning,
// not a rejection.
func (UndefinedOperatorPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if root == nil {
		return nil
	}
	var diags []diag.Diagnostic
	ast.Inspect(root, func(n ast.Node) bool {
		if e, ok := n.(*ast.OperatorExpr); ok && e.Operator == ast.OpBitNot {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Span:     e.Span(),
				Message:  msgUndefinedOperator,
				Code:     codeUndefinedOperator,
				Source:   "syntax",
			})
		}
		return true
	})
	return diags
}
