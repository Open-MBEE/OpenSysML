package solve

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// valueTypedVariation returns the variation point a term of a variation sort
// reads, when every variant of it declares a value rather than standing for an
// object: `==` on such a point compares the values its variants declare
// (KerML BaseFunctions::'==' is value equality for data values), as the
// evaluator does. A point whose variants are objects, or an enumeration, is
// nil; a point mixing the two refuses.
func (t *translator) valueTypedVariation(node ast.Node, construct string, term *Term) (*symbols.Symbol, error) {
	if term.Sort.Kind != SortDatatype || !term.Sort.Variation {
		return nil, nil
	}
	variation := t.origins[term.Sort.Name]
	if variation == nil {
		return nil, nil
	}
	valued, objects := 0, 0
	for _, variant := range t.model.VariantsOf(variation) {
		if semantics.VariantValue(variant) != nil {
			valued++
		} else {
			objects++
		}
	}
	switch {
	case valued == 0:
		return nil, nil
	case objects > 0:
		return nil, t.refuse(node, construct, "the variants of "+variation.Name+" declare values and objects both")
	}
	return variation, nil
}

// selectionValue is the value a term of a value-typed variation sort stands
// for: a variant, the value it declares; the point itself, the value of
// whichever variant it selects, each variant's value translated under its
// selection, as the evaluator reads only the selected one.
func (t *translator) selectionValue(node ast.Node, construct string, variation *symbols.Symbol, term *Term) (*Term, error) {
	variants := t.model.VariantsOf(variation)
	if term.Op == OpValue {
		for _, variant := range variants {
			if term.Str == t.fqn(variant) {
				return t.expr(semantics.VariantValue(variant), variant.OwnerScope)
			}
		}
		return nil, t.refuse(node, construct, term.Str+" is not a variant of "+variation.Name)
	}
	values := make([]*Term, 0, len(variants))
	t.branched++
	defer func() { t.branched-- }()
	for _, variant := range variants {
		value, err := t.under(Binary(OpEq, Bool, term, ValueTerm(term.Sort, t.fqn(variant))), func() (*Term, error) {
			return t.expr(semantics.VariantValue(variant), variant.OwnerScope)
		})
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	sort := values[0].Sort
	for _, value := range values[1:] {
		if value.Sort.Kind == SortReal && sort.Kind == SortInt {
			sort = Real
		}
	}
	for i, value := range values {
		if value.Sort.Kind == SortInt && sort.Kind == SortReal {
			value = ToReal(value)
		}
		if !value.Sort.Equal(sort) {
			return nil, t.refuse(node, construct, "the variants of "+variation.Name+" declare values of "+sort.Name+" and "+value.Sort.Name)
		}
		values[i] = value
	}
	selected := values[len(values)-1]
	for i := len(values) - 2; i >= 0; i-- {
		chosen := Binary(OpEq, Bool, term, ValueTerm(term.Sort, t.fqn(variants[i])))
		selected = Ite(chosen, values[i], selected)
	}
	return selected, nil
}

// choice reports whether a term of a variation sort denotes a variant itself —
// a variant named, or a conditional choosing among variants — rather than the
// value a variation point holds. Two choices compare as choices.
func choice(term *Term) bool {
	switch term.Op {
	case OpValue:
		return true
	case OpIte:
		return len(term.Args) == 3 && choice(term.Args[1]) && choice(term.Args[2])
	}
	return false
}

// equalityOf builds `==` or `!=` of two translated operands. Two variants compare
// as the choice itself; a value-typed variation point, or one of its variants
// against anything else, compares by the value it stands for.
func (t *translator) equalityOf(n ast.Node, scope *symbols.Scope, construct string, op Op, left, right *Term) (*Term, error) {
	if !choice(left) || !choice(right) {
		var err error
		if left, err = t.asSelectionValue(n, construct, left); err != nil {
			return nil, err
		}
		if right, err = t.asSelectionValue(n, construct, right); err != nil {
			return nil, err
		}
	}
	left, right = promote(left, right)
	if !left.Sort.Equal(right.Sort) {
		return nil, t.refuse(n, construct, "its operands yield "+left.Sort.Name+" and "+right.Sort.Name)
	}
	if left.Sort.Numeric() {
		if cmp, ok := n.(*ast.OperatorExpr); ok {
			if err := t.sameDimension(cmp, scope); err != nil {
				return nil, err
			}
		}
	}
	return Binary(op, Bool, left, right), nil
}

// asSelectionValue replaces a term of a value-typed variation sort by the value
// it stands for, leaving any other term as it is.
func (t *translator) asSelectionValue(node ast.Node, construct string, term *Term) (*Term, error) {
	variation, err := t.valueTypedVariation(node, construct, term)
	if err != nil || variation == nil {
		return term, err
	}
	return t.selectionValue(node, construct, variation, term)
}
