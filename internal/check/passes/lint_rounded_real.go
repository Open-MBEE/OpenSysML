package passes

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// CodeRoundedRealLiteral marks a decimal literal bound or assigned to a Real-typed
// feature that no binary64 Real equals, so the feature holds the nearest one.
const CodeRoundedRealLiteral = "rounded-real-literal"

// holdsReal reports whether the feature d declares is a Real one: typed Real as
// written, else by the effective types it inherits (KerML §8.3.3.3).
func (ec *exprChecker) holdsReal(declScope *symbols.Scope, d featureDecl, written semantics.PrimType) bool {
	if written != semantics.PrimUnknown {
		return written == semantics.PrimReal
	}
	for _, typ := range ec.model.FeatureTypes(declaredSymbol(declScope, d.node)) {
		if prim := ec.model.PrimTypeOf(typ); prim != semantics.PrimUnknown {
			return prim == semantics.PrimReal
		}
	}
	return false
}

// lintRoundedReal reports value, bound to a Real-typed feature, when it is a decimal
// literal, signed or not, whose exact Rational no binary64 Real equals.
func (ec *exprChecker) lintRoundedReal(value ast.Node) {
	literal, sign := value, ""
	if op, ok := value.(*ast.OperatorExpr); ok && len(op.Operands) == 1 &&
		(op.Operator == ast.OpNeg || op.Operator == ast.OpPos) {
		literal = op.Operands[0]
		if op.Operator == ast.OpNeg {
			sign = "-"
		}
	}
	lit, ok := literal.(*ast.LiteralReal)
	if !ok {
		return
	}
	exact, err := semantics.ParseRational(lit.Value, semantics.DefaultMaxIntegerBits)
	if err != nil {
		return
	}
	// A literal beyond the Real range is the run time's to refuse.
	real, err := semantics.RealOf(exact)
	if err != nil || new(big.Rat).SetFloat64(real.Real).Cmp(exact.Rat()) == 0 {
		return
	}
	ec.lint(CodeRoundedRealLiteral, value.Span(),
		"%s%s is rounded to the nearest Real, %s%s, since a feature typed by Real holds a binary64 value; type the feature by Rational to keep the literal exact",
		sign, lit.Value, sign, distinctDigits(real.Real, exact))
}

// distinctDigits writes x with the fewest significant digits, 17 at least, that
// tell it from exact, which it rounds.
func distinctDigits(x float64, exact semantics.Value) string {
	text := strconv.FormatFloat(x, 'g', 17, 64)
	for digits := 18; digits <= 767; digits++ {
		if parsed, err := semantics.ParseRational(text, semantics.DefaultMaxIntegerBits); err != nil || semantics.CompareRat(parsed, exact) != 0 {
			break
		}
		text = strconv.FormatFloat(x, 'g', digits, 64)
	}
	return text
}

// lint records a lint finding of code at span.
func (ec *exprChecker) lint(code string, span source.Span, format string, args ...any) {
	ec.diags = append(ec.diags, diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     span,
		Message:  fmt.Sprintf(format, args...),
		Code:     code,
		Source:   lintSource,
	})
}
