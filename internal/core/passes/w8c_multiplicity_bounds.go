package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// KerMLValidator's validateMultiplicityRangeBoundResultTypes message.
const msgMultiplicityBoundNatural = "Must have a Natural value"

// MultiplicityBoundsPass checks that every multiplicity bound has a Natural
// value (KerML 8.3.3.1.9, validateMultiplicityRangeBoundResultTypes): a
// model-level evaluable bound must evaluate to a non-negative whole number or
// `*`, and any other bound must have an Integer-conforming result type.
type MultiplicityBoundsPass struct{}

func (MultiplicityBoundsPass) Level() PassLevel { return LevelConstraint }

func (MultiplicityBoundsPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &multiplicityBoundsChecker{resolver: ctx.Resolver(), model: ctx.Model()}
	w := &kit.Walker{Ctx: ctx}
	w.Walk(rootScope, c.check)
	return c.diags
}

type multiplicityBoundsChecker struct {
	resolver *resolve.Resolver
	model    *semantics.Model
	diags    []diag.Diagnostic
}

func (c *multiplicityBoundsChecker) check(sym *symbols.Symbol) {
	mult := kit.MultiplicityOf(sym)
	if mult == nil {
		return
	}
	scope := kit.DeclarationScope(sym)
	c.checkBound(scope, mult.Lower)
	if mult.IsRange {
		c.checkBound(scope, mult.Upper)
	}
}

func (c *multiplicityBoundsChecker) checkBound(scope *symbols.Scope, bound ast.Node) {
	if bound == nil {
		return
	}
	if v, ok := c.model.EvalIn(scope, bound); ok {
		if v.Kind == semantics.ValInfinity || (v.Kind == semantics.ValInt && v.Int >= 0) {
			return
		}
		c.report(bound)
		return
	}
	if !c.boundIsInteger(scope, bound) {
		c.report(bound)
	}
}

// boundIsInteger reports whether a bound's result may be an Integer: a known
// result type must conform to Integer (a class-typed feature does not), while
// an unresolved or untyped bound is left to the tiers that own those gaps.
func (c *multiplicityBoundsChecker) boundIsInteger(scope *symbols.Scope, bound ast.Node) bool {
	return c.boundConforms(scope, bound, semantics.PrimInteger)
}

// boundConforms reports whether a bound's result may conform to want (Integer or
// Natural); `**` and `^` keep an Integer base whole only under a Natural exponent.
func (c *multiplicityBoundsChecker) boundConforms(scope *symbols.Scope, bound ast.Node, want semantics.PrimType) bool {
	switch e := bound.(type) {
	case *ast.LiteralInteger, *ast.LiteralInfinity:
		return true
	case *ast.OperatorExpr:
		if e.Operator == ast.OpPow && len(e.Operands) == 2 {
			return c.boundConforms(scope, e.Operands[0], want) && c.boundConforms(scope, e.Operands[1], semantics.PrimNatural)
		}
		if w8cWholeOperator(e.Operator, want) {
			for _, arg := range e.Operands {
				if !c.boundConforms(scope, arg, want) {
					return false
				}
			}
			return true
		}
	}
	silent := &exprChecker{resolver: c.resolver, model: c.model}
	if !kit.IsReference(bound) {
		if prim := silent.infer(scope, bound); prim != semantics.PrimUnknown {
			return semantics.PrimConforms(prim, want)
		}
		// A call, constructor or quantity is judged by its declared result.
		conf := c.model.ExprConformsTo(scope, bound, c.model.ScalarSymbol(want))
		return !conf.Known || conf.Holds || conf.Untyped
	}
	sym, ok := c.resolver.ResolveTarget(scope, bound)
	if !ok || sym == nil {
		return true
	}
	if prim := silent.featurePrimType(sym); prim != semantics.PrimUnknown {
		return semantics.PrimConforms(prim, want)
	}
	return !c.model.DeclaresResolvedType(sym, want)
}

// w8cWholeOperator reports the operators closed over want: division may yield a
// Rational, and subtraction or negation may leave the Naturals.
func w8cWholeOperator(op ast.OperatorKind, want semantics.PrimType) bool {
	switch op {
	case ast.OpAdd, ast.OpMul, ast.OpMod, ast.OpPos:
		return true
	case ast.OpSub, ast.OpNeg:
		return want != semantics.PrimNatural
	}
	return false
}

func (c *multiplicityBoundsChecker) report(bound ast.Node) {
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     bound.Span(),
		Message:  msgMultiplicityBoundNatural,
		Code:     "multiplicity-bound-natural",
		Source:   "constraint",
	})
}
