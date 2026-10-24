package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// PositiveScalar reports a type that is, or specializes, ScalarValues::Positive.
func (ctx *Context) PositiveScalar(typ *symbols.Symbol) bool {
	positive := ctx.librarySymbol("ScalarValues::Positive")
	return positive != nil && ctx.modelConforms(typ, positive)
}

// QuantityNumber is the feature holding a scalar quantity type's magnitude, the
// `num` a quantity value's number is bound to; false for any other type.
func (ctx *Context) QuantityNumber(typ *symbols.Symbol) (*symbols.Symbol, bool) {
	scalar := ctx.librarySymbol(scalarQuantityTypeFQN)
	if scalar == nil || !ctx.modelConforms(typ, scalar) {
		return nil, false
	}
	num, ok := ctx.model.semantics.LookupMember(typ, vectorQuantityNumFeature)
	if !ok || num == nil {
		return nil, false
	}
	return num, true
}

// Parameter is one input parameter of a calc or case as a tool binding its
// arguments reads it: its name, whether it is a case's subject, and the
// declaration a bound value answers to.
type Parameter struct {
	Name      string
	IsSubject bool
	decl      calcMemberDecl
}

// Type is the type the parameter declares, nil where it declares none.
func (p Parameter) Type() *symbols.Symbol {
	if p.decl.Target == nil {
		return nil
	}
	return p.decl.Target.typ
}

// Owner is the behavior declaring the parameter, in whose scope its values are classified.
func (p Parameter) Owner() *symbols.Symbol { return p.decl.Owner }

// Admit checks the value as an argument bound to the parameter is, naming it by what.
func (p Parameter) Admit(ctx *Context, value *Value, what func() string) error {
	return p.decl.check(ctx, value, what)
}

// Parameters are the input parameters the calc or case declares, the subject
// included, in declaration order.
func (ctx *Context) Parameters(sym *symbols.Symbol) ([]Parameter, error) {
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return nil, err
	}
	params := make([]Parameter, len(shape.Params))
	for i := range shape.Params {
		p := &shape.Params[i]
		params[i] = Parameter{Name: p.Name, IsSubject: p.IsSubject, decl: p.Decl}
	}
	return params, nil
}

// ClassifyValue reports whether the value, read in scope, is of the target type
// by any of the types it carries.
func (ctx *Context) ClassifyValue(scope *symbols.Scope, value Value, target *symbols.Symbol) (semantics.TypeClassification, error) {
	return ctx.classifyValue(scope, value, target, nil, byAnyType)
}
