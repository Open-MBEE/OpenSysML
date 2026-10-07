package codegen

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// An unset value is the interpreter's materialized value of a required scalar
// feature nothing was written to (runtime.Context.HoldsNoValue): one value,
// not null, with an identity of its own, failing wherever a concrete value is
// required. A compiled value that may be unset has a type that MayUnset.

// widenUnset records that slot s may hold an unset value.
func (c *Compiler) widenUnset(s slot) {
	if !c.unsets[s] {
		c.unsets[s], c.widened = true, true
	}
}

// need is v as a concrete operand, failing as the interpreter's valueOperand
// does when v is unset.
func (fc *funcCompiler) need(v Expr) Expr {
	if !v.Type().MayUnset() {
		return v
	}
	feature := "an expression"
	if nm, ok := v.(Named); ok {
		feature = nm.Feature
	}
	return Need{X: v, Fail: fmt.Sprintf("%v for feature %s", runtime.ErrNoValue, feature)}
}

// unsetUnsupported refuses v, which may be unset, where what needs a value
// the compiled subset cannot hold unset.
func (fc *funcCompiler) unsetUnsupported(what string) error {
	return fc.unsupported(fmt.Sprintf("a value that may be %s as %s", runtime.UnsetText, what))
}

// liftTo is v as a value of t, which MayUnset.
func (fc *funcCompiler) liftTo(v Expr, t Type, what string) (Expr, error) {
	if !v.Type().MayUnset() {
		x, err := fc.coerce(v, t.Concrete(), what)
		if err != nil {
			return nil, err
		}
		return Lift{X: x, T: t}, nil
	}
	vb, tb := v.Type().Concrete(), t.Concrete()
	var conv func(Expr) Expr
	switch {
	case vb == tb:
		return v, nil
	case tb == TypeNum && (vb == TypeInt || vb == TypeReal):
		conv = func(x Expr) Expr { return ToNum{X: x} }
	case tb == TypeReal && (vb == TypeInt || vb == TypeNum):
		conv = func(x Expr) Expr { return ToReal{X: x} }
	default:
		return nil, fc.unsupported(fmt.Sprintf("a %s %s where %s is expected", vb, what, tb))
	}
	tmp, lets := fc.hoistVar(v, nil)
	return wrapLets(lets, Relabel{V: conv(Strip{X: tmp}), Of: tmp, T: t}), nil
}

// bindUnset is bind of v where v or b may be unset; ok is false when neither is.
func (fc *funcCompiler) bindUnset(v Expr, b binding, where, label string) (Expr, bool, error) {
	vt := v.Type()
	switch {
	case !vt.MayUnset() && !b.t.MayUnset():
		return nil, false, nil
	case !vt.MayUnset():
		concrete := b
		concrete.t = b.t.Concrete()
		x, err := fc.bind(v, concrete, where, label)
		if err != nil {
			return nil, true, err
		}
		return Lift{X: x, T: b.t}, true, nil
	case b.t.Many():
		return nil, true, fc.unsetUnsupported(fmt.Sprintf("a value bound at %s, which holds a collection", where))
	}
	t := b.t
	if !t.MayUnset() {
		if b.slot == nil {
			return nil, true, fc.unsetUnsupported(fmt.Sprintf("a value bound at %s, which holds %s", where, b.t))
		}
		fc.c.widenUnset(*b.slot)
		t = t.Unsettable()
	}
	if t.Concrete() == TypeReal && vt.Concrete() != TypeReal && b.slot != nil {
		// An Integer written to a Real-typed feature stays an Integer.
		fc.c.widen(*b.slot)
	}
	x, err := fc.liftTo(v, t, "bound at "+where)
	return x, true, err
}

// unsetBranches types `if` branches of which one may be unset.
func (fc *funcCompiler) unsetBranches(then, els Expr) (Expr, Expr, Type, error) {
	tt, et := then.Type().Concrete(), els.Type().Concrete()
	var t Type
	switch {
	case !tt.Scalar() || !et.Scalar():
		return nil, nil, TypeInvalid, fc.unsetUnsupported("a branch of if whose other branch is not one value")
	case tt == et:
		t = tt
	case numeric(tt) && numeric(et):
		t = TypeNum
	default:
		return nil, nil, TypeInvalid, fc.unsupported(fmt.Sprintf("branches of if are %s and %s", tt, et))
	}
	t = t.Unsettable()
	then, err := fc.liftTo(then, t, "branch of if")
	if err != nil {
		return nil, nil, TypeInvalid, err
	}
	els, err = fc.liftTo(els, t, "branch of if")
	if err != nil {
		return nil, nil, TypeInvalid, err
	}
	return then, els, t, nil
}

// unsetEquality is `==`, `!=`, `===` or `!==` with an operand that may be
// unset: an unset value equals only itself, under every one of them.
func (fc *funcCompiler) unsetEquality(eq func(l, r Expr) (Expr, error), l, r Expr, neq bool) (Expr, error) {
	if l.Type().Many() || r.Type().Many() || l.Type() == TypeNull || r.Type() == TypeNull {
		if l.Type() != TypeNull && r.Type() != TypeNull {
			return nil, fc.unsetUnsupported("an operand of equality with a collection")
		}
		// One value, unset or not, is not null.
		var result Expr = BoolLit{Value: neq}
		for _, x := range []Expr{r, l} {
			if x.Type() != TypeNull {
				result = wrapLets([]Let{{Name: fc.temp(x.Type()).Name, Value: x}}, result)
				continue
			}
			n, rest := leading(x)
			if _, ok := rest.(NullLit); !ok {
				return nil, fc.unsetUnsupported("an operand of equality with a null that is not the literal null")
			}
			result = charged(n, result)
		}
		return result, nil
	}
	var lets []Let
	l, lets = fc.hoistVar(l, lets)
	r, lets = fc.hoistVar(r, lets)
	ls, rs := l, r
	var unset Expr
	var same Expr = BoolLit{Value: neq}
	switch {
	case l.Type().MayUnset() && r.Type().MayUnset():
		ls, rs = Strip{X: l}, Strip{X: r}
		unset = Binary{Op: ast.OpConditionalOr, L: IsUnset{X: l}, R: IsUnset{X: r}, T: TypeBool}
		same = SameUnset{L: l, R: r, Neq: neq}
	case l.Type().MayUnset():
		ls, unset = Strip{X: l}, IsUnset{X: l}
	default:
		rs, unset = Strip{X: r}, IsUnset{X: r}
	}
	base, err := eq(ls, rs)
	if err != nil {
		return nil, err
	}
	return wrapLets(lets, Cond{C: unset, Then: same, Else: base, T: TypeBool}), nil
}

// hoistVar is v evaluated into a fresh temporary unless it is already a Var.
func (fc *funcCompiler) hoistVar(v Expr, lets []Let) (Expr, []Let) {
	if _, ok := v.(Var); ok {
		return v, lets
	}
	tmp := fc.temp(v.Type())
	return Var{Name: tmp.Name, T: tmp.Type}, append(lets, Let{Name: tmp.Name, Value: v})
}

// unsetTag is the low bits of an unset identity of a feature narrowed by r:
// 2 more than the least Integer r admits, 1 for any Integer.
func unsetTag(r Range) int64 {
	if r == RangeAny {
		return 1
	}
	return r.Lower() + 2
}
