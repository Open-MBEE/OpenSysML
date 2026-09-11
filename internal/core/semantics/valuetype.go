package semantics

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Library types an expression's value is classified against, and the scalar
// types a literal is of.
const (
	FQNDurationValue    = "ISQBase::DurationValue"
	FQNTimeInstantValue = "Time::TimeInstantValue"
	FQNBoolean          = "ScalarValues::Boolean"

	fqnString                     = "ScalarValues::String"
	fqnNatural                    = "ScalarValues::Natural"
	fqnPositive                   = "ScalarValues::Positive"
	fqnInteger                    = "ScalarValues::Integer"
	fqnRational                   = "ScalarValues::Rational"
	fqnReal                       = "ScalarValues::Real"
	fqnNumericalValue             = "ScalarValues::NumericalValue"
	fqnAnything                   = "Base::Anything"
	fqnEvaluation                 = "Performances::Evaluation"
	fqnBooleanEvaluation          = "Performances::BooleanEvaluation"
	fqnCollection                 = "Collections::Collection"
	fqnMetaobject                 = "Metaobjects::Metaobject"
	fqnTensorMeasurementReference = "MeasurementReferences::TensorMeasurementReference"
	fqnTensorQuantityValue        = "Quantities::TensorQuantityValue"
)

// Conformance is the outcome of judging an expression's value against a type.
// A value whose type only evaluation determines is Known false, and a rule that
// consumes the judgement must then stay silent rather than guess.
type Conformance struct {
	Known bool
	Holds bool
	// Found describes what the value is, for a diagnostic when Holds is false.
	Found string
	// Untyped marks a value whose static type says nothing — a feature declaring
	// no type, a result typed Anything — so a rule may leave it to evaluation.
	Untyped bool
}

func conformanceUnknown() Conformance { return Conformance{} }

// ExprConformsToLibrary is ExprConformsTo against the library type named by fqn,
// unknown when the library declaring it is not loaded.
func (m *Model) ExprConformsToLibrary(scope *symbols.Scope, node ast.Node, fqn string) Conformance {
	if m == nil {
		return conformanceUnknown()
	}
	return m.ExprConformsTo(scope, node, m.libSymbol(fqn))
}

// TimeEventType names the library type a time trigger's argument must be a value
// of: a DurationValue after `after`, a TimeInstantValue after `at`.
func TimeEventType(t *ast.TimeEvent) string {
	if t.Absolute {
		return FQNTimeInstantValue
	}
	return FQNDurationValue
}

// TimeEventConforms judges a time trigger's argument against TimeEventType, the
// one judgement validation and execution share.
func (m *Model) TimeEventConforms(scope *symbols.Scope, t *ast.TimeEvent) Conformance {
	if t == nil || t.Duration == nil {
		return conformanceUnknown()
	}
	return m.ExprConformsToLibrary(scope, t.Duration, TimeEventType(t))
}

// ExprConformsTo reports whether an expression's value conforms to want, as far
// as the declarations the expression names determine it statically: a literal by
// its scalar type, a feature by its declared type, a quantity literal by the unit
// it is written in, an invocation by its result, an arithmetic expression by the
// dimension of its value, `null` as the empty Anything. A feature declared by
// nothing but a value is typed by that value (KerML
// checkFeatureValuationSpecialization); a declared type is never narrowed by
// the value.
func (m *Model) ExprConformsTo(scope *symbols.Scope, node ast.Node, want *symbols.Symbol) Conformance {
	return m.exprConformance(scope, node, want, true)
}

// exprConformance is ExprConformsTo, judging a quantity literal and quantity
// arithmetic by the unit written when byUnit is set and otherwise by the
// ScalarQuantityValue result the quantity functions declare.
func (m *Model) exprConformance(scope *symbols.Scope, node ast.Node, want *symbols.Symbol, byUnit bool) Conformance {
	if m == nil || node == nil || want == nil || m.resolver == nil {
		return conformanceUnknown()
	}
	if alias, ok := m.resolver.ResolveAliasTarget(want); ok {
		want = alias
	}
	switch n := node.(type) {
	case *ast.LiteralBool:
		return m.typeConformance(m.libSymbol(FQNBoolean), want)
	case *ast.LiteralString:
		return m.typeConformance(m.libSymbol(fqnString), want)
	case *ast.LiteralInteger:
		return m.typeConformance(m.libSymbol(fqnNatural), want)
	case *ast.LiteralReal:
		return m.typeConformance(m.libSymbol(fqnRational), want)
	case *ast.LiteralInfinity:
		// `*` is the natural number exceeding every other (KerML 8.4.4.6).
		return m.typeConformance(m.libSymbol(fqnPositive), want)
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		sym, ok := m.resolver.ResolveTarget(scope, n)
		if !ok || sym == nil {
			return conformanceUnknown()
		}
		return m.featureConformance(sym, want)
	case *ast.IndexExpr:
		if !n.Bracket {
			return m.indexConformance(scope, n, want, byUnit)
		}
		if !byUnit {
			return m.typeConformance(m.quantityValueType(scope, n), want)
		}
		return m.quantityConformance(scope, n, want)
	case *ast.OperatorExpr:
		return m.operatorConformance(scope, n, want, byUnit)
	case *ast.InvocationExpr:
		return m.invocationConformance(scope, n, want, byUnit)
	case *ast.NullExpr:
		// `null` evaluates to nothing, typed Anything (KerML nullEvaluations).
		c := m.typeConformance(m.libSymbol(fqnAnything), want)
		if c.Known && !c.Holds {
			c.Found = "`null`, an empty value typed Anything"
			c.Untyped = true
		}
		return c
	case *ast.SequenceExpr:
		// `a, b` is the result of BaseFunctions::',', typed Anything.
		c := m.typeConformance(m.libSymbol(fqnAnything), want)
		if c.Known && !c.Holds {
			c.Found = fmt.Sprintf("a sequence of %d elements, typed Anything", len(n.Elements))
			c.Untyped = true
		}
		return c
	case *ast.CollectExpr:
		// `xs.{…}` is the result of ControlFunctions::collect: the body's results, else Anything.
		if c, ok := m.collectionConformance(scope, n, want, byUnit); ok {
			return c
		}
		c := m.typeConformance(m.libSymbol(fqnAnything), want)
		if c.Known && !c.Holds {
			c.Found = "a collection `.{…}` maps to, typed Anything"
			c.Untyped = true
		}
		return c
	case *ast.SelectExpr:
		// `xs.?{…}` keeps elements of xs (KerML checkSelectExpressionResultSpecialization).
		if c, ok := m.collectionConformance(scope, n, want, byUnit); ok {
			return c
		}
		return m.exprConformance(scope, n.Operand, want, byUnit)
	case *ast.BodyExpr:
		c := m.typeConformance(m.bodyExprType(scope, n), want)
		if c.Known && !c.Holds {
			c.Found = "an expression body `{ … }`, the expression itself"
		}
		return c
	case *ast.ConstructorExpr:
		if n.Type == nil {
			return conformanceUnknown()
		}
		def, ok := m.resolver.ResolveQualified(scope, n.Type)
		if !ok || def == nil {
			return conformanceUnknown()
		}
		if alias, ok := m.resolver.ResolveAliasTarget(def); ok {
			def = alias
		}
		return m.typeConformance(def, want)
	}
	return conformanceUnknown()
}

// ExprResultType is the type of an expression's result: that of the feature it
// names, else what its syntax declares (literal, operator, constructor); nil if unknown.
func (m *Model) ExprResultType(scope *symbols.Scope, node ast.Node) *symbols.Symbol {
	if m == nil {
		return nil
	}
	switch n := node.(type) {
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		if m.resolver == nil {
			return nil
		}
		sym, ok := m.resolver.ResolveTarget(scope, n)
		if !ok || sym == nil {
			return nil
		}
		return m.featureResultType(sym)
	case *ast.LiteralBool:
		return m.libSymbol(FQNBoolean)
	case *ast.LiteralString:
		return m.libSymbol(fqnString)
	case *ast.LiteralInteger:
		return m.libSymbol(fqnInteger)
	case *ast.LiteralReal:
		return m.libSymbol(fqnReal)
	case *ast.OperatorExpr:
		if n.Operator == ast.OpAs {
			// `x as T` results in T (KerML checkCastExpressionResultSpecialization).
			return m.namedType(scope, n.TypeRef)
		}
		if types := m.extentTypes(scope, n); len(types) > 0 {
			// `all T` results in instances of T (KerML 1.0 §7.4.9.2, BaseFunctions::'all').
			return types[0]
		}
		if unit := m.MeasurementRefExprType(scope, n); unit != nil {
			return unit
		}
		if frame := m.CoordinateFrameExprType(scope, n); frame != nil {
			return frame
		}
		return m.libSymbol(operatorResultFQN(n.Operator))
	case *ast.IndexExpr:
		if n.Bracket {
			return m.libSymbol(fqnAnything)
		}
		return m.indexResultType(scope, n)
	case *ast.SelectExpr:
		// `xs.?{…}` keeps elements of xs (KerML checkSelectExpressionResultSpecialization).
		if types := m.selectResultTypes(scope, n); len(types) > 0 {
			return types[0]
		}
		return m.ExprResultType(scope, n.Operand)
	case *ast.CollectExpr:
		// `xs.{…}` is the result of ControlFunctions::collect: the body's results, else Anything.
		if types := m.collectResultTypes(scope, n); len(types) > 0 {
			return types[0]
		}
		return m.libSymbol(fqnAnything)
	case *ast.NullExpr, *ast.SequenceExpr:
		return m.libSymbol(fqnAnything)
	case *ast.BodyExpr:
		return m.bodyExprType(scope, n)
	case *ast.ConstructorExpr:
		return m.namedType(scope, n.Type)
	case *ast.InvocationExpr:
		called := m.invocationCallee(scope, n)
		if types := m.collectionResultTypes(scope, n, called); len(types) > 0 {
			return types[0]
		}
		if result := m.ResultParameterOf(called); result != nil {
			return m.featureResultType(result)
		}
	}
	return nil
}

// bodyExprType is the type of `{ … }` written as a value: the expression itself, an
// Evaluation — a BooleanEvaluation when its result is Boolean, as the pilot reads it.
func (m *Model) bodyExprType(scope *symbols.Scope, body *ast.BodyExpr) *symbols.Symbol {
	if body.Result != nil {
		inner := symbols.BodyExprScope(scope, body)
		if c := m.ExprConformsTo(inner, body.Result, m.libSymbol(FQNBoolean)); c.Known && c.Holds {
			return m.libSymbol(fqnBooleanEvaluation)
		}
	}
	return m.libSymbol(fqnEvaluation)
}

// namedType resolves a type reference, following an alias; nil if unresolved.
func (m *Model) namedType(scope *symbols.Scope, qn *ast.QualifiedName) *symbols.Symbol {
	if qn == nil || m.resolver == nil {
		return nil
	}
	def, ok := m.resolver.ResolveQualified(scope, qn)
	if !ok || def == nil {
		return nil
	}
	if alias, ok := m.resolver.ResolveAliasTarget(def); ok {
		def = alias
	}
	return def
}

// operatorResultFQN names the result type of the Kernel Function Library
// function an operator resolves to (BaseFunctions, DataFunctions, ControlFunctions).
func operatorResultFQN(op ast.OperatorKind) string {
	switch op {
	case ast.OpEq, ast.OpNeq, ast.OpEqEqEq, ast.OpNeqEqEq, ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe,
		ast.OpIsType, ast.OpHasType, ast.OpAt, ast.OpMetaAt,
		ast.OpConditionalAnd, ast.OpConditionalOr, ast.OpImplies:
		return FQNBoolean
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpPow, ast.OpMod, ast.OpNeg, ast.OpPos,
		ast.OpNot, ast.OpAnd, ast.OpOr, ast.OpXor, ast.OpBitNot, ast.OpRange:
		return dataValueFQN
	case ast.OpMeta:
		return fqnMetaobject
	}
	return fqnAnything
}

// indexResultType is the type of one element `seq#(i)` selects: seq's own type,
// or Anything when seq is a Collection (KerML checkIndexExpressionResultSpecialization).
func (m *Model) indexResultType(scope *symbols.Scope, n *ast.IndexExpr) *symbols.Symbol {
	seq := m.ExprResultType(scope, n.Operand)
	if collection := m.libSymbol(fqnCollection); seq == nil || collection == nil || m.Conforms(seq, collection) {
		return m.libSymbol(fqnAnything)
	}
	return seq
}

// featureResultType is a feature's declared type, else that of the value typing
// it (KerML checkFeatureValuationSpecialization), else what it redefines or subsets has.
func (m *Model) featureResultType(sym *symbols.Symbol) *symbols.Symbol {
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = alias
	}
	if !sym.IsFeature() {
		return nil
	}
	return m.inheritedResultType(sym, map[*symbols.Symbol]bool{})
}

// MeasurementRefFeatureType is the measurement unit type of a feature, declared or
// taken from its value (`attribute area = m * m;` is a DerivedUnit); nil for any other.
func (m *Model) MeasurementRefFeatureType(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || m.resolver == nil || sym == nil {
		return nil
	}
	typ := m.featureResultType(sym)
	if typ == nil || !m.IsMeasurementUnit(typ) {
		return nil
	}
	return typ
}

func (m *Model) inheritedResultType(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) *symbols.Symbol {
	if seen[sym] {
		return nil
	}
	seen[sym] = true
	var features []*symbols.Symbol
	bases := m.implicitBases(sym)
	for _, super := range m.DirectSupertypes(sym) {
		if alias, ok := m.resolver.ResolveAliasTarget(super); ok {
			super = alias
		}
		switch {
		case containsElement(bases, super):
		case super.Kind.IsDefinition():
			return super
		case super.IsFeature():
			features = append(features, super)
		}
	}
	// A self-referential value types nothing.
	if value := m.typingValue(sym); value != nil && !m.valuing[sym] {
		m.valuing[sym] = true
		defer delete(m.valuing, sym)
		return m.ExprResultType(sym.OwnerScope, value)
	}
	for _, f := range features {
		if typ := m.inheritedResultType(f, seen); typ != nil {
			return typ
		}
	}
	return nil
}

// indexConformance judges `seq#(i)` as one element of seq, of seq's type — or as
// Anything when seq is a Collection (KerML checkIndexExpressionResultSpecialization).
func (m *Model) indexConformance(scope *symbols.Scope, n *ast.IndexExpr, want *symbols.Symbol, byUnit bool) Conformance {
	collection := m.libSymbol(fqnCollection)
	if collection == nil {
		return conformanceUnknown()
	}
	seq := m.exprConformance(scope, n.Operand, collection, byUnit)
	if !seq.Known {
		return conformanceUnknown()
	}
	if !seq.Holds {
		return m.exprConformance(scope, n.Operand, want, byUnit)
	}
	c := m.typeConformance(m.libSymbol(fqnAnything), want)
	if c.Known && !c.Holds {
		c.Found = "an element `#` selects from a Collection (every quantity value is one), typed Anything"
		c.Untyped = true
	}
	return c
}

// typeConformance judges a value known to be of typ.
func (m *Model) typeConformance(typ, want *symbols.Symbol) Conformance {
	if typ == nil {
		return conformanceUnknown()
	}
	return Conformance{Known: true, Holds: m.Conforms(typ, want), Found: leafName(typ.Name)}
}

// featureConformance judges a value that is what a feature holds: the feature's
// declared type bounds it. A feature with a generalization that does not resolve
// has an undetermined type, reported elsewhere.
func (m *Model) featureConformance(sym, want *symbols.Symbol) Conformance {
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = alias
	}
	if !sym.IsFeature() {
		return conformanceUnknown()
	}
	if m.Conforms(sym, want) {
		return Conformance{Known: true, Holds: true}
	}
	// A step named as a value stands for its result: a constraint is its Boolean.
	if result := m.ResultParameterOf(sym); result != nil && result != sym {
		return m.featureConformance(result, want)
	}
	if value := m.typingValue(sym); value != nil {
		return m.valueConformance(sym, value, want)
	}
	if !m.generalizationsResolve(sym) {
		return conformanceUnknown()
	}
	if typ := m.nearestDeclaredType(sym); typ != nil {
		return Conformance{Known: true, Found: leafName(typ.Name)}
	}
	return Conformance{Known: true, Found: "an untyped feature", Untyped: true}
}

// typingValue returns the value a feature is implicitly typed by: a non-default
// value of a usage declaring no generalization and no direction (KerML §8.3.3.3
// checkFeatureValuationSpecialization), else nil.
func (m *Model) typingValue(sym *symbols.Symbol) ast.Node {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || u.Value == nil || u.ValueIsDefault || u.Direction != ast.DirNone {
		return nil
	}
	for _, rel := range u.Relationships {
		if rel != nil && GeneralizationKind(rel.Kind) {
			return nil
		}
	}
	return u.Value
}

// valueConformance judges a feature by the result of the value it is typed by,
// resolved where the declaration was written; a quantity result is the
// ScalarQuantityValue its function declares, so a duration literal types a
// feature as a quantity, not a DurationValue. A value that leads back to its
// own feature has no static type.
func (m *Model) valueConformance(sym *symbols.Symbol, value ast.Node, want *symbols.Symbol) Conformance {
	if m.valuing[sym] {
		return conformanceUnknown()
	}
	m.valuing[sym] = true
	defer delete(m.valuing, sym)
	return m.exprConformance(sym.OwnerScope, value, want, false)
}

// generalizationsResolve reports whether every generalization a declaration
// states names an element.
func (m *Model) generalizationsResolve(sym *symbols.Symbol) bool {
	for _, rel := range RelationshipsOf(sym) {
		if !GeneralizationKind(rel.Kind) || rel.Target == nil {
			continue
		}
		if m.relationshipTarget(sym, rel) == nil {
			return false
		}
	}
	return true
}

// nearestDeclaredType returns the type a feature is declared with, or the one a
// feature it redefines or subsets is; nil for a feature typed by nothing but
// its kind's base.
func (m *Model) nearestDeclaredType(sym *symbols.Symbol) *symbols.Symbol {
	if defs := m.declaredTypes(sym, map[*symbols.Symbol]bool{}); len(defs) > 0 {
		return defs[0]
	}
	return nil
}

// declaredTypes returns the definitions a feature is directly typed by, or those
// of the features it redefines or subsets when it restates none itself. The
// base its kind implies (`Base::dataValues`) types every feature of the kind
// and so says nothing about this one; nor does the parameter of its owner's
// kind base that a parameter implicitly redefines (`Performance::result`).
func (m *Model) declaredTypes(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) []*symbols.Symbol {
	if seen[sym] {
		return nil
	}
	seen[sym] = true
	var defs, features []*symbols.Symbol
	bases := m.implicitBases(sym)
	baseParam := m.implicitBaseParameter(sym)
	for _, super := range m.DirectSupertypes(sym) {
		if alias, ok := m.resolver.ResolveAliasTarget(super); ok {
			super = alias
		}
		switch {
		case super == baseParam, slices.Contains(bases, super):
		case super.Kind.IsDefinition():
			defs = append(defs, super)
		case super.IsFeature():
			features = append(features, super)
		}
	}
	if len(defs) > 0 {
		return defs
	}
	for _, f := range features {
		defs = append(defs, m.declaredTypes(f, seen)...)
	}
	return defs
}

// implicitBaseParameter returns the parameter sym implicitly redefines in the
// kind base of its owning behavior or step, or nil when there is none.
func (m *Model) implicitBaseParameter(sym *symbols.Symbol) *symbols.Symbol {
	if sym.OwnerScope == nil || sym.OwnerScope.Owner() == nil {
		return nil
	}
	for _, base := range m.implicitBases(sym.OwnerScope.Owner()) {
		params := m.parametersOf(base)
		for _, redefined := range m.ImplicitParameterRedefinitions(sym) {
			if redefined == params.result.sym {
				return redefined
			}
			for _, p := range params.positional {
				if redefined == p.sym {
					return redefined
				}
			}
		}
	}
	return nil
}

// quantityConformance judges a magnitude paired with a measurement reference
// (`5 [s]`), a ScalarQuantityValue — or numbers paired with a coordinate frame
// (`(1, 2, 3) [cf]`, `(1, 2, 3) [cf / s]`), a VectorQuantityValue. Against a
// quantity value type it is one of that type when its reference is of the kind the
// type's `mRef` admits — a DurationValue is written in a DurationUnit, a composed
// frame is judged as `cf / s` is — or, when the reference is not a single named
// one, when its dimension is the type's.
func (m *Model) quantityConformance(scope *symbols.Scope, n *ast.IndexExpr, want *symbols.Symbol) Conformance {
	quantity := m.quantityValueType(scope, n)
	if quantity == nil {
		return conformanceUnknown()
	}
	ref := m.measurementReference(scope, n.Index)
	composed, isComposed := m.composedFrameIndex(scope, n.Index)
	if !m.Conforms(want, quantity) {
		c := m.typeConformance(quantity, want)
		if ref != nil {
			c.Found = m.describeQuantity(ref)
		} else if isComposed {
			c.Found = "a vector quantity in " + UnitExprText(n.Index)
		}
		return c
	}
	if ref == nil && !isComposed {
		return m.dimensionConformance(scope, n, want)
	}
	admitted := m.AdmittedMeasurementRefs(want)
	if len(admitted) == 0 {
		return conformanceUnknown()
	}
	if isComposed {
		for _, typ := range admitted {
			if c := m.ComposedFrameConforms(composed, typ); !c.Known || !c.Holds {
				c.Found = "a vector quantity in " + UnitExprText(n.Index) + ", " + c.Found
				return c
			}
		}
		return Conformance{Known: true, Holds: true}
	}
	holds := true
	for _, typ := range admitted {
		holds = holds && m.Conforms(ref, typ)
	}
	return Conformance{Known: true, Holds: holds, Found: m.describeQuantity(ref)}
}

// AdmittedMeasurementRefs is what a quantity value type's `mRef` is typed by: the
// references a quantity of that type may be written in; nil where it declares none.
func (m *Model) AdmittedMeasurementRefs(typ *symbols.Symbol) []*symbols.Symbol {
	if m == nil || typ == nil {
		return nil
	}
	mRef, ok := m.LookupMember(typ, memberMRef)
	if !ok || mRef == nil {
		return nil
	}
	return m.declaredTypes(mRef, map[*symbols.Symbol]bool{})
}

// FramedQuantityConformance judges numbers written in a coordinate frame, named or
// composed (`(1, 2, 3) [cf]`, `(1, 2, 3) [cf / s]`), against want; false when the
// literal's index is not a frame.
func (m *Model) FramedQuantityConformance(scope *symbols.Scope, n *ast.IndexExpr, want *symbols.Symbol) (Conformance, bool) {
	if m == nil || n == nil || !n.Bracket || n.Index == nil || want == nil || m.resolver == nil {
		return conformanceUnknown(), false
	}
	if m.quantityValueType(scope, n) != m.libSymbol(fqnVectorQuantityValue) {
		return conformanceUnknown(), false
	}
	if alias, ok := m.resolver.ResolveAliasTarget(want); ok {
		want = alias
	}
	return m.quantityConformance(scope, n, want), true
}

// quantityValueType is the type a quantity literal has: a VectorQuantityValue
// over a coordinate frame, named or composed (VectorCalculations::'['), else a
// ScalarQuantityValue.
func (m *Model) quantityValueType(scope *symbols.Scope, n *ast.IndexExpr) *symbols.Symbol {
	if ref := m.measurementReference(scope, n.Index); m.IsVectorReference(ref) {
		return m.libSymbol(fqnVectorQuantityValue)
	}
	if _, ok := m.composedFrameIndex(scope, n.Index); ok {
		return m.libSymbol(fqnVectorQuantityValue)
	}
	return m.libSymbol(fqnScalarQuantityValue)
}

// composedFrameIndex is the frame an index composes (`(1, 2, 3) [spatialCF / s]`);
// false for a named reference or a unit expression.
func (m *Model) composedFrameIndex(scope *symbols.Scope, index ast.Node) (ComposedFrame, bool) {
	e, ok := index.(*ast.OperatorExpr)
	if !ok {
		return ComposedFrame{}, false
	}
	return m.composedFrameExpr(scope, e)
}

// describeQuantity names a quantity by the measurement reference it is written in.
func (m *Model) describeQuantity(ref *symbols.Symbol) string {
	found := fmt.Sprintf("a quantity in %s", leafName(ref.Name))
	if m.IsVectorReference(ref) {
		found = fmt.Sprintf("a vector quantity in %s", leafName(ref.Name))
	}
	if typ := m.nearestDeclaredType(ref); typ != nil {
		found += fmt.Sprintf(" (a %s)", leafName(typ.Name))
	}
	return found
}

// measurementReference returns the measurement reference a quantity literal
// names when it is a single resolved one, or nil for a derived unit expression.
func (m *Model) measurementReference(scope *symbols.Scope, unit ast.Node) *symbols.Symbol {
	sym, ok := m.resolver.ResolveTarget(scope, unit)
	if !ok || sym == nil {
		return nil
	}
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = alias
	}
	reference := m.libSymbol(fqnTensorMeasurementReference)
	if reference == nil || !m.Conforms(sym, reference) || !m.generalizationsResolve(sym) {
		return nil
	}
	return sym
}

// operatorConformance judges an operator's value by the result of the library
// function its operands select: comparisons are Boolean, a conditional the
// Anything its function returns, `%` a number; arithmetic is a quantity when
// every operand is one (QuantityCalculations; a power when its base is and its
// exponent a Real), else a number, `+` over strings a String, and `*`/`/` of a
// coordinate frame by a unit the frame MeasurementRefCalculations compose.
func (m *Model) operatorConformance(scope *symbols.Scope, e *ast.OperatorExpr, want *symbols.Symbol, byUnit bool) Conformance {
	switch e.Operator {
	case ast.OpAll:
		// `all T` holds instances of T (KerML 1.0 §7.4.9.2): judged as the collection it is.
		if c, ok := m.collectionConformance(scope, e, want, byUnit); ok {
			return c
		}
	case ast.OpConditional, ast.OpNullCoalesce:
		c := m.typeConformance(m.libSymbol(fqnAnything), want)
		if c.Known && !c.Holds {
			c.Found = fmt.Sprintf("the result of `%s`, typed Anything", e.Operator)
			c.Untyped = true
		}
		return c
	case ast.OpMod:
		if m.operandsConformTo(scope, e, m.libSymbol(fqnNumericalValue)) {
			return m.typeConformance(m.libSymbol(fqnNumericalValue), want)
		}
	case ast.OpNot, ast.OpAnd, ast.OpOr, ast.OpXor, ast.OpConditionalAnd, ast.OpConditionalOr,
		ast.OpImplies, ast.OpEq, ast.OpNeq, ast.OpEqEqEq, ast.OpNeqEqEq,
		ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe, ast.OpIsType, ast.OpHasType, ast.OpAt, ast.OpMetaAt:
		return m.typeConformance(m.libSymbol(FQNBoolean), want)
	case ast.OpNeg, ast.OpPos, ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpPow:
		if m.quantityArithmetic(scope, e) {
			if byUnit {
				return m.dimensionConformance(scope, e, want)
			}
			return m.typeConformance(m.libSymbol(fqnScalarQuantityValue), want)
		}
		if m.operandsConformTo(scope, e, m.libSymbol(fqnNumericalValue)) {
			return m.typeConformance(m.libSymbol(fqnNumericalValue), want)
		}
		if e.Operator == ast.OpAdd && m.operandsConformTo(scope, e, m.libSymbol(fqnString)) {
			return m.typeConformance(m.libSymbol(fqnString), want)
		}
		if c, ok := m.MeasurementRefExprConformance(scope, e, want); ok {
			return c
		}
		if c, ok := m.CoordinateFrameExprConformance(scope, e, want); ok {
			return c
		}
		if found, ok := m.nonArithmeticOperand(scope, e); ok {
			return Conformance{Known: true, Found: fmt.Sprintf("`%s` over %s, which no arithmetic function takes", e.Operator, found)}
		}
	}
	return conformanceUnknown()
}

// nonArithmeticOperand finds an operand known to be neither a quantity, a number
// nor a String, so no arithmetic function is selected and e has no result type.
func (m *Model) nonArithmeticOperand(scope *symbols.Scope, e *ast.OperatorExpr) (string, bool) {
	for _, operand := range e.Operands {
		found := ""
		for _, fqn := range []string{fqnScalarQuantityValue, fqnNumericalValue, fqnString} {
			typ := m.libSymbol(fqn)
			if typ == nil {
				return "", false
			}
			c := m.exprConformance(scope, operand, typ, false)
			if !c.Known || c.Holds || c.Untyped {
				found = ""
				break
			}
			found = c.Found
		}
		if found != "" {
			return found, true
		}
	}
	return "", false
}

// quantityArithmetic reports whether e selects a QuantityCalculations function:
// every operand is a quantity, or for a power the base is and the exponent a Real.
func (m *Model) quantityArithmetic(scope *symbols.Scope, e *ast.OperatorExpr) bool {
	quantity := m.libSymbol(fqnScalarQuantityValue)
	if e.Operator != ast.OpPow || len(e.Operands) != 2 {
		return m.operandsConformTo(scope, e, quantity)
	}
	return m.operandConformsTo(scope, e.Operands[0], quantity) && m.operandConformsTo(scope, e.Operands[1], m.libSymbol(fqnReal))
}

// operandsConformTo reports whether every operand is known to be a value of
// typ, so the expression is the result the function over typ declares.
func (m *Model) operandsConformTo(scope *symbols.Scope, e *ast.OperatorExpr, typ *symbols.Symbol) bool {
	if len(e.Operands) == 0 {
		return false
	}
	for _, operand := range e.Operands {
		if !m.operandConformsTo(scope, operand, typ) {
			return false
		}
	}
	return true
}

// operandConformsTo reports whether operand is known to be a value of typ.
func (m *Model) operandConformsTo(scope *symbols.Scope, operand ast.Node, typ *symbols.Symbol) bool {
	if typ == nil {
		return false
	}
	c := m.exprConformance(scope, operand, typ, false)
	return c.Known && c.Holds
}

// dimensionConformance judges an expression whose value is a quantity of a
// determined dimension: against a quantity value type by that dimension, against
// any other type as the ScalarQuantityValue it is. A dimensionless value may be
// a plain number, so only a quantity value type judges it.
func (m *Model) dimensionConformance(scope *symbols.Scope, node ast.Node, want *symbols.Symbol) Conformance {
	got, ok := m.DimensionOfExpr(scope, node)
	if !ok {
		if sum, lhs, rhs, incommensurable := m.incommensurableSum(scope, node); incommensurable {
			return Conformance{Known: true, Found: fmt.Sprintf("`%s` over incommensurable quantities of dimension %s and %s", sum.Operator, lhs, rhs)}
		}
		return conformanceUnknown()
	}
	found := "a value of dimension " + got.String()
	if got.Term.Dimensionless() {
		found = "a dimensionless value"
	}
	quantity := m.libSymbol(fqnScalarQuantityValue)
	if quantity != nil && !m.Conforms(want, quantity) {
		if got.Term.Dimensionless() {
			return conformanceUnknown()
		}
		c := m.typeConformance(quantity, want)
		c.Found = found
		return c
	}
	wantDim, ok := m.DimensionOfType(want)
	if !ok {
		return conformanceUnknown()
	}
	return Conformance{Known: true, Holds: got.Term.Commensurable(wantDim.Term), Found: found}
}

// QuantityConforms judges a value in a reduced unit against a declared type: a
// quantity value type by dimension, any other type as a ScalarQuantityValue.
func (m *Model) QuantityConforms(unit UnitTerm, want *symbols.Symbol) Conformance {
	if m == nil || want == nil {
		return conformanceUnknown()
	}
	got, ok := m.DimensionOfUnit(unit)
	if !ok {
		return conformanceUnknown()
	}
	found := "a value of dimension " + got.String()
	if got.Term.Dimensionless() {
		found = "a dimensionless value"
	}
	quantity := m.libSymbol(fqnScalarQuantityValue)
	if quantity == nil {
		return conformanceUnknown()
	}
	if !m.Conforms(want, quantity) {
		c := m.typeConformance(quantity, want)
		c.Found = found
		return c
	}
	wantDim, ok := m.DimensionOfType(want)
	if !ok {
		return Conformance{Known: true, Holds: true, Found: found}
	}
	return Conformance{Known: true, Holds: got.Term.Commensurable(wantDim.Term), Found: found}
}

// MeasurementRefConforms judges a measurement reference against a declared type:
// by the type it is declared with (typ, DerivedUnit for a composed unit), or
// else by dimension when want is a unit definition fixing one, as `m*m` is an AreaUnit.
// A scale (TimeScale) fixes a dimension too, but no unit is a scale.
func (m *Model) MeasurementRefConforms(typ *symbols.Symbol, unit UnitTerm, want *symbols.Symbol) Conformance {
	if m == nil || typ == nil || want == nil {
		return conformanceUnknown()
	}
	c := m.typeConformance(typ, want)
	if !c.Known || c.Holds {
		return c
	}
	c.Found = "a measurement reference typed " + c.Found
	if !m.IsMeasurementUnit(want) {
		return c
	}
	wantDim, ok := m.dimensionOf(want)
	if !ok {
		return c
	}
	got, ok := m.dimensionOfUnitTerm(unit)
	if !ok {
		return c
	}
	c.Found = "a measurement reference of dimension " + Dimension{Term: got}.String()
	c.Holds = got.Commensurable(wantDim)
	return c
}

// MeasurementRefExprConformance judges a product, quotient or power of measurement
// units (`m * s`), the DerivedUnit such a composition is; false for any other expression.
func (m *Model) MeasurementRefExprConformance(scope *symbols.Scope, e *ast.OperatorExpr, want *symbols.Symbol) (Conformance, bool) {
	if want == nil {
		return conformanceUnknown(), false
	}
	unit, term, ok := m.measurementRefExpr(scope, e)
	if !ok {
		return conformanceUnknown(), false
	}
	return m.MeasurementRefConforms(unit, term, want), true
}

// MeasurementRefExprType is the type of a product, quotient or power of measurement
// units: DerivedUnit, a unit of powers of other units; nil for any other expression.
// A power by a Real not known statically is one too, of a dimension the runtime finds.
func (m *Model) MeasurementRefExprType(scope *symbols.Scope, e *ast.OperatorExpr) *symbols.Symbol {
	if unit, _, ok := m.measurementRefExpr(scope, e); ok {
		return unit
	}
	if m.measurementRefPower(scope, e) {
		return m.libSymbol(fqnDerivedUnit)
	}
	return nil
}

// measurementRefPower reports whether e raises a measurement unit to an exponent
// that is a Real but folds to no number.
func (m *Model) measurementRefPower(scope *symbols.Scope, e *ast.OperatorExpr) bool {
	if m == nil || e == nil || e.Operator != ast.OpPow || len(e.Operands) != 2 {
		return false
	}
	if _, ok := m.measurementRefOperand(scope, e.Operands[0]); !ok {
		return false
	}
	return m.operandConformsTo(scope, e.Operands[1], m.libSymbol(fqnReal))
}

// measurementRefExpr reduces `*`, `/` or `**` over measurement units to the unit it
// composes, with the DerivedUnit type; false when e is not such an expression.
func (m *Model) measurementRefExpr(scope *symbols.Scope, e *ast.OperatorExpr) (*symbols.Symbol, UnitTerm, bool) {
	if m == nil || e == nil {
		return nil, UnitTerm{}, false
	}
	unit := m.libSymbol(fqnDerivedUnit)
	if unit == nil {
		return nil, UnitTerm{}, false
	}
	term, ok := m.measurementRefOperand(scope, e)
	if !ok {
		return nil, UnitTerm{}, false
	}
	return unit, term, true
}

// measurementRefOperand reduces a unit's name, `*`/`/` of two such, or `**` of one
// by a number; a number itself (`1 * 1`) is none, though unit notation reads `1`.
func (m *Model) measurementRefOperand(scope *symbols.Scope, node ast.Node) (UnitTerm, bool) {
	switch n := node.(type) {
	case *ast.FeatureReference, *ast.QualifiedName:
		term, err := m.UnitTermOfExpr(scope, n)
		return term, err == nil
	case *ast.OperatorExpr:
		if len(n.Operands) != 2 {
			return UnitTerm{}, false
		}
		base, ok := m.measurementRefOperand(scope, n.Operands[0])
		if !ok {
			return UnitTerm{}, false
		}
		switch n.Operator {
		case ast.OpMul, ast.OpDiv:
			other, ok := m.measurementRefOperand(scope, n.Operands[1])
			if !ok {
				return UnitTerm{}, false
			}
			if n.Operator == ast.OpMul {
				return base.Times(other), true
			}
			return base.DividedBy(other), true
		case ast.OpPow:
			exp, ok := m.Eval(n.Operands[1])
			if !ok || !exp.IsNumeric() {
				return UnitTerm{}, false
			}
			return base.Pow(exp.AsReal()), true
		}
	}
	return UnitTerm{}, false
}

// incommensurableSum finds, in quantity arithmetic, a sum or difference of
// operands whose dimensions are both known and incommensurable. Such an
// expression has no value of any dimension, so it is a value of no quantity type.
func (m *Model) incommensurableSum(scope *symbols.Scope, node ast.Node) (*ast.OperatorExpr, Dimension, Dimension, bool) {
	e, ok := node.(*ast.OperatorExpr)
	if !ok {
		return nil, Dimension{}, Dimension{}, false
	}
	switch e.Operator {
	case ast.OpAdd, ast.OpSub:
		if lhs, rhs, ok := m.operandDimensions(scope, e); ok && !lhs.Term.Commensurable(rhs.Term) {
			return e, lhs, rhs, true
		}
	case ast.OpNeg, ast.OpPos, ast.OpMul, ast.OpDiv, ast.OpPow:
	default:
		return nil, Dimension{}, Dimension{}, false
	}
	for _, operand := range e.Operands {
		if sum, lhs, rhs, ok := m.incommensurableSum(scope, operand); ok {
			return sum, lhs, rhs, true
		}
	}
	return nil, Dimension{}, Dimension{}, false
}

// invocationConformance judges an invocation's value by the declared type of the
// result parameter of the overload it calls.
func (m *Model) invocationConformance(scope *symbols.Scope, e *ast.InvocationExpr, want *symbols.Symbol, byUnit bool) Conformance {
	called := m.invocationCallee(scope, e)
	if c, ok := m.collectionConformance(scope, e, want, byUnit); ok {
		return c
	}
	result := m.ResultParameterOf(called)
	if result == nil {
		return conformanceUnknown()
	}
	return m.featureConformance(result, want)
}

// invocationCallee is the declaration e calls, as the checker selects it; nil
// when the call is unresolved, ambiguous or fits none.
func (m *Model) invocationCallee(scope *symbols.Scope, e *ast.InvocationExpr) *symbols.Symbol {
	if e.Type == nil || m.resolver == nil {
		return nil
	}
	sym, ok := m.calledFunction(scope, e)
	if !ok {
		return nil
	}
	return sym
}

// invocationResult is the result parameter of the declaration e calls; nil when
// the call is unresolved, ambiguous or fits none.
func (m *Model) invocationResult(scope *symbols.Scope, e *ast.InvocationExpr) *symbols.Symbol {
	return m.ResultParameterOf(m.invocationCallee(scope, e))
}
