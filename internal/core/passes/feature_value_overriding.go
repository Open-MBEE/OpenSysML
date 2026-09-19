package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// featureValue is the value part of a feature declaration: the span from its
// operator through its expression, and whether it is a `default`
// (KerML FeatureValue::isDefault).
type featureValue struct {
	span      source.Span
	isDefault bool
}

// featureValueOf returns the value part declared on sym, if any.
func featureValueOf(sym *symbols.Symbol) (featureValue, bool) {
	if oc, ok := ast.OwnedConstraintOf(sym.Decl); ok {
		if oc.Value == nil {
			return featureValue{}, false
		}
		return featureValue{span: valuePartSpan(oc.ValueOperatorSpan, oc.Value), isDefault: oc.ValueIsDefault}, true
	}
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		if d.Value == nil {
			return featureValue{}, false
		}
		return featureValue{span: valuePartSpan(d.ValueOperatorSpan, d.Value), isDefault: d.ValueIsDefault}, true
	case *ast.SubjectMember:
		if d.BindingExpr == nil {
			return featureValue{}, false
		}
		return featureValue{span: valuePartSpan(d.ValueOperatorSpan, d.BindingExpr), isDefault: d.ValueIsDefault}, true
	}
	return featureValue{}, false
}

// valuePartSpan covers a value part from its operator through its expression.
func valuePartSpan(op source.Span, value ast.Node) source.Span {
	span := value.Span()
	if op.Len == 0 || op.Offset > span.Offset {
		return span
	}
	return source.Span{Offset: op.Offset, Len: span.End() - op.Offset}
}

// checkFeatureValueOverriding flags a feature value on a feature that redefines,
// directly or transitively, a feature bound with `=` rather than `default`
// (KerML 1.0 §8.3.4.10.2 FeatureValue, validateFeatureValueOverriding).
func (cc *constraintChecker) checkFeatureValueOverriding(sym *symbols.Symbol) {
	value, ok := featureValueOf(sym)
	if !ok {
		return
	}
	for _, redefined := range cc.model.AllRedefinedFeatures(sym) {
		bound, ok := featureValueOf(redefined)
		if !ok || bound.isDefault {
			continue
		}
		name := cc.featureName(redefined)
		cc.diags = append(cc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     value.span,
			Message: fmt.Sprintf(
				"cannot override the binding value of %s: a value written with `=` is fixed for every feature redefining it; "+
					"write it as `default =` on %s to allow overriding, or remove this value",
				name, name),
			Code:   "feature-value-overriding",
			Source: "constraint",
		})
		return
	}
}

// featureName is the qualified name the index knows sym by, or its own name;
// an unnamed feature (a `return` result) is named by its owner.
func (cc *constraintChecker) featureName(sym *symbols.Symbol) string {
	if sym.Name == "" {
		if sym.OwnerScope != nil && sym.OwnerScope.Owner() != nil {
			return "the unnamed feature of " + cc.featureName(sym.OwnerScope.Owner())
		}
		return "an unnamed feature"
	}
	if cc.resolver != nil && cc.resolver.Index() != nil {
		if fqn := cc.resolver.Index().GetFQN(sym); fqn != "" {
			return fqn
		}
	}
	return sym.Name
}
