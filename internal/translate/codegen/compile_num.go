package codegen

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A number is a value of a Real-typed feature: the interpreter keeps an Integer
// written to one an Integer, so it prints, compares under `===` and computes as
// one. Arithmetic over numbers splits at run time on whether every operand
// holds an Integer.

// split evaluates operands in order, then applies ints when every number among
// them holds an Integer, else reals; both see each operand as one Expr.
func (fc *funcCompiler) split(operands []Expr, ints, reals func([]Expr) Expr, t Type) Expr {
	var lets []Let
	var nums []Var
	vs := make([]Expr, len(operands))
	for i, x := range operands {
		if x.Type() == TypeNum {
			if _, ok := x.(Var); !ok {
				lets = append(lets, Let{Name: fc.temp(TypeNum).Name, Value: x})
				x = Var{Name: lets[len(lets)-1].Name, T: TypeNum}
			}
			nums = append(nums, x.(Var))
		} else {
			x, lets = fc.hoist(x, lets)
		}
		vs[i] = x
	}
	var out Expr = NumSplit{Nums: nums, Int: ints(vs), Real: reals(vs), T: t}
	for i := len(lets) - 1; i >= 0; i-- {
		out = Let{Name: lets[i].Name, Value: lets[i].Value, In: out}
	}
	return out
}

// asInt is x, an Integer or a number found to hold one, as an Integer.
func asInt(x Expr) Expr {
	if x.Type() == TypeNum {
		return AsInt{X: x}
	}
	return x
}

// asReal is x as a Real.
func asReal(x Expr) Expr {
	if x.Type() == TypeReal {
		return x
	}
	return ToReal{X: x}
}

// numArith applies the arithmetic operator op to scalar operands at least one
// of which is a number: over a Real it is Real arithmetic; otherwise it is
// Integer arithmetic when both hold Integers and Real arithmetic when not.
func (fc *funcCompiler) numArith(op ast.OperatorKind, l, r Expr) (Expr, error) {
	if l.Type() == TypeReal || r.Type() == TypeReal {
		return Binary{Op: op, L: asReal(l), R: asReal(r), T: TypeReal}, nil
	}
	reals := func(v []Expr) Expr { return Binary{Op: op, L: asReal(v[0]), R: asReal(v[1]), T: TypeReal} }
	switch op {
	case ast.OpDiv:
		// A quotient of Integers is a Rational.
		return fc.split([]Expr{l, r}, func(v []Expr) Expr {
			return Binary{Op: op, L: asInt(v[0]), R: asInt(v[1]), T: TypeReal}
		}, reals, TypeReal), nil
	case ast.OpPow:
		if lit, ok := bare(r).(IntLit); ok && lit.sign() < 0 {
			return Binary{Op: op, L: asReal(l), R: asReal(r), T: TypeReal}, nil
		}
		// Integer ** Integer is an Integer for a non-negative exponent, a Real otherwise.
		return fc.split([]Expr{l, r}, func(v []Expr) Expr {
			ints := ToNum{X: Binary{Op: op, L: asInt(v[0]), R: asInt(v[1]), T: TypeInt}}
			if lit, ok := bare(v[1]).(IntLit); ok && lit.sign() >= 0 {
				return ints
			}
			return Cond{
				C:    Binary{Op: ast.OpGe, L: asInt(v[1]), R: IntLit{}, T: TypeBool},
				Then: ints,
				Else: ToNum{X: reals(v)},
				T:    TypeNum,
			}
		}, func(v []Expr) Expr { return ToNum{X: reals(v)} }, TypeNum), nil
	}
	return fc.split([]Expr{l, r}, func(v []Expr) Expr {
		return ToNum{X: Binary{Op: op, L: asInt(v[0]), R: asInt(v[1]), T: TypeInt}}
	}, func(v []Expr) Expr { return ToNum{X: reals(v)} }, TypeNum), nil
}
