package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// writeTarget is the declaration a written value answers to: the type the
// feature written was declared with, and the multiplicity governing how many
// values it holds.
type writeTarget struct {
	name string
	typ  *symbols.Symbol
	mult semantics.Range
}

// admission is how an object written to a feature answers to the feature's type: a declared value
// or binding classifies the object it holds (KerML 1.0 §7.3.4.1); a write or message takes it as it is.
type admission uint8

const (
	admitWritten admission = iota
	admitDeclared
)

// writeTargetKey identifies a target by where it was written and the name it
// wrote, which resolve to one declaration however often the write runs.
type writeTargetKey struct {
	scope *symbols.Scope
	name  string
}

// writeTargetIn resolves the feature an assignment names in the scope the
// statement was written in, memoized per scope and name. A name declaring no
// feature there — a value the body merely holds — states nothing to conform to.
func (ctx *Context) writeTargetIn(scope *symbols.Scope, name string) (*writeTarget, bool) {
	if ctx.resolver == nil || scope == nil || name == "" {
		return nil, false
	}
	key := writeTargetKey{scope: scope, name: name}
	if cached, ok := ctx.writeTargets[key]; ok {
		return cached, cached != nil
	}
	var target *writeTarget
	if sym, ok := ctx.resolver.LookupName(scope, name); ok && sym != nil && semantics.IsShapeFeature(sym) {
		mult, _ := ctx.extractMultiplicity(sym)
		target = &writeTarget{name: name, typ: ctx.extractType(sym), mult: mult}
	}
	if ctx.writeTargets == nil {
		ctx.writeTargets = make(map[writeTargetKey]*writeTarget)
	}
	ctx.writeTargets[key] = target
	return target, target != nil
}

// checkWrite reports a value that does not conform to the declaration of the
// feature written. KerML's FeatureWritePerformance "assigns the values of a
// feature on an occurrence to the given replacementValues", so those values are
// values of that feature and answer to its type and its multiplicity — the rule
// binding an initial value (passes.checkBoundValue, Context.checkDefaultCount),
// applied where a write replaces them.
func (ctx *Context) checkWrite(scope *symbols.Scope, what string, target *writeTarget, value Value) error {
	if target == nil {
		return nil
	}
	if msg := ctx.writeCountRefusal(target, &value); msg != "" {
		return fmt.Errorf("%s: %w: %s", what, ErrMultiplicityViolation, msg)
	}
	return ctx.checkWriteType(scope, what, target.typ, value, admitWritten)
}

// writeCountRefusal says why the number of values written is outside the
// target's multiplicity, or is empty where the count is admitted.
func (ctx *Context) writeCountRefusal(target *writeTarget, value *Value) string {
	return target.mult.CountViolation(elementCount(value))
}

// checkBodyWrite checks a write of a value a behavior body itself holds - a
// block-local, a parameter, an output - against the declaration of the name it
// writes, before that value is stored.
func (ctx *Context) checkBodyWrite(host stmtHost, s lower.Assign, value Value) error {
	return ctx.checkNamedWrite(s.Scope, host.describe(), s.Target, value)
}

// checkNamedWrite checks a write of a name resolved in scope, for a path that
// stores the value itself rather than reaching Instance.SetFeatureValue.
func (ctx *Context) checkNamedWrite(scope *symbols.Scope, where, name string, value Value) error {
	return ctx.checkBoundName(scope, fmt.Sprintf("%s: assignment to %s", where, name), name, value)
}

// checkBoundName checks a value bound to the feature name declares in scope,
// described by what: an assignment, or a binding that gives a value to an
// output the run time computes.
func (ctx *Context) checkBoundName(scope *symbols.Scope, what, name string, value Value) error {
	target, ok := ctx.writeTargetIn(scope, name)
	if !ok {
		return nil
	}
	return ctx.checkWrite(scope, what, target, value)
}

// storeBodyValue writes a value into the behavior's own data once it conforms
// to the declaration of the name written.
func storeBodyValue(ctx *Context, host stmtHost, env *stmtEnv, name string, value Value, s lower.Assign) error {
	if err := ctx.checkBodyWrite(host, s, value); err != nil {
		return err
	}
	env.data.set(name, value)
	return nil
}

// checkWriteType reports an element of a written value that no feature of the
// declared type could hold. A target declaring no type holds anything, and a
// value whose type the run time cannot name is not judged here.
func (ctx *Context) checkWriteType(scope *symbols.Scope, what string, declared *symbols.Symbol, value Value, how admission) error {
	if refusal, refused := ctx.writeTypeRefusal(scope, declared, &value, how); refused {
		return fmt.Errorf("%s: %w: %s", what, ErrTypeMismatch, refusal)
	}
	return nil
}

// writeTypeRefusal says why the first element no feature of the declared type
// could hold is refused; the second result is false where every element conforms.
func (ctx *Context) writeTypeRefusal(scope *symbols.Scope, declared *symbols.Symbol, value *Value, how admission) (string, bool) {
	if declared == nil {
		return "", false
	}
	switch value.Kind {
	case ValSequence, ValSet:
		elements := elementsOf(*value)
		for i := range elements {
			if refusal, refused := ctx.elementRefusal(scope, declared, &elements[i], how); refused {
				return refusal, true
			}
		}
		return "", false
	}
	return ctx.elementRefusal(scope, declared, value, how)
}

// elementRefusal says why one element is refused by a feature of the declared type.
func (ctx *Context) elementRefusal(scope *symbols.Scope, declared *symbols.Symbol, element *Value, how admission) (string, bool) {
	conforms, refusal, err := ctx.valueConforms(scope, element, declared, how)
	if err != nil || conforms {
		return "", false
	}
	if refusal == "" {
		refusal = fmt.Sprintf("cannot write %s (%s) to a feature typed by %s",
			FormatValue(*element), describeValue(*element), symbolText(declared))
	}
	return refusal, true
}

// valueConforms reports whether a feature of the declared type may hold the
// value, by the relation the type tier applies to an initial value: the scalar
// lattice where the target is a scalar type, specialization otherwise. The
// second result says why a value was refused where the general message would
// not say it, and is empty otherwise.
func (ctx *Context) valueConforms(scope *symbols.Scope, value *Value, declared *symbols.Symbol, how admission) (bool, string, error) {
	switch value.Kind {
	case ValNull, ValInvalid:
		// Holds no value to type; how many values a feature may hold is the
		// multiplicity's to decide.
		return true, "", nil
	case ValQuantity:
		return ctx.quantityConforms(*value, declared)
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity:
		return ctx.structuredConforms(scope, *value, declared, how)
	case ValMeasurementRef:
		return ctx.measurementRefConforms(value.MeasurementRef(), declared)
	case ValCoordinateFrame:
		return ctx.frameConforms(value.CoordinateFrame(), declared)
	case ValCoordinateTransformation:
		return ctx.transformationConforms(value.CoordinateTransformation(), declared)
	}
	if id, ok := value.Object(); ok {
		// An object, a selected variant's included, is what its usage is; declared as a
		// value of a narrower type, it is classified by that type too (see classify.go).
		inst, ok := ctx.instances[id]
		if !ok || inst == nil || inst.Type == nil {
			return false, "", fmt.Errorf("%w: instance %d", ErrUndeterminedValueType, id)
		}
		if how == admitDeclared {
			return ctx.canClassify(inst, declared), "", nil
		}
		return ctx.instanceConforms(inst, declared), "", nil
	}
	// A scalar is what its representation and the declared type say (classifyValue); a
	// verdict they leave open is the declaration's to make (KerML 1.0 §7.3.4.1).
	verdict, err := ctx.classifyValue(scope, *value, declared, nil, byAnyType)
	if err != nil {
		return false, "", err
	}
	return verdict != semantics.ClassifiesNone, "", nil
}

// isScalarConstant reports a value written as one scalar constant.
func isScalarConstant(value *Value) bool {
	return value.Kind == ValConst || value.Kind == ValComplex || value.Kind == ValString
}

// isStructuredValue reports an array, vector, vector quantity or tensor quantity value.
func isStructuredValue(value *Value) bool {
	switch value.Kind {
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity:
		return true
	}
	return false
}

// structuredConforms judges a written array, vector or vector/tensor quantity: its type's
// supertypes, or a non-scalar specialization of its base type whose shape it fits.
func (ctx *Context) structuredConforms(scope *symbols.Scope, value Value, declared *symbols.Symbol, how admission) (bool, string, error) {
	direct, err := ctx.structuredValueType(value)
	if err != nil {
		return false, "", err
	}
	if !ctx.model.Conforms(direct, declared) {
		base, err := ctx.structuredBaseType(value)
		if err != nil {
			return false, "", err
		}
		if !ctx.model.Conforms(declared, base) {
			return false, "", nil
		}
		if scalar := ctx.librarySymbol(scalarValueTypeFQN); scalar != nil && ctx.model.Conforms(declared, scalar) {
			return false, fmt.Sprintf("cannot write %s (%s) to a feature typed by %s: it is a %s, which holds one scalar",
				FormatValue(value), describeValue(value), symbolText(declared), symbolText(scalar)), nil
		}
		if refusal := ctx.shapeRefusal(value, declared); refusal != "" {
			return false, refusal, nil
		}
		if ok, refusal, err := ctx.elementsConform(scope, value, declared, how); !ok || err != nil {
			return ok, refusal, err
		}
	}
	switch value.Kind {
	case ValVectorQuantity:
		vq := value.VectorQuantity()
		for i := 0; i < vq.Dimension(); i++ {
			if ok, refusal, err := ctx.quantityConforms(NewQuantityValue(vq.component(i)), declared); !ok || err != nil {
				return ok, refusal, err
			}
		}
		if vq.Frame != nil {
			if ok, refusal := ctx.framedQuantityConforms(value, declared); !ok {
				return false, refusal, nil
			}
		}
	case ValTensorQuantity:
		return ctx.tensorQuantityConforms(value.TensorQuantity(), declared)
	}
	return true, "", nil
}

// shapeRefusal says why a value's dimensions violate the shape a type fixes: a
// constant on an Array shape feature or its redefinition, or `dimensions`' multiplicity.
func (ctx *Context) shapeRefusal(value Value, declared *symbols.Symbol) string {
	dims := structuredDimensions(value)
	size, _ := flattenedSize(dims)
	refuse := func(declares string) string {
		return fmt.Sprintf("cannot write %s (%s) to a feature typed by %s: it declares %s",
			FormatValue(value), describeValue(value), symbolText(declared), declares)
	}
	for _, member := range ctx.model.MembersOfIncludingRedefined(declared) {
		if !semantics.IsShapeFeature(member) {
			continue
		}
		feature, ok := ctx.arrayFeatureOf(member)
		if !ok || feature == arrayElementsFeature {
			continue
		}
		if feature == arrayDimensionsFeature {
			if mult, stated := ctx.model.MultiplicityOf(member); stated && mult.CountViolation(int64(len(dims))) != "" {
				return refuse(fmt.Sprintf("%s : Positive%s, got %d dimension(s)", member.Name, mult.Text(), len(dims)))
			}
		}
		want, ok := ctx.constantIntegers(member, declared)
		if !ok {
			continue
		}
		stated := FormatValue(intSequence(want))
		if len(want) == 1 {
			stated = fmt.Sprint(want[0])
		}
		declares := fmt.Sprintf("%s = %s", member.Name, stated)
		switch feature {
		case arrayDimensionsFeature:
			if !equalInt64s(want, dims) {
				return refuse(declares)
			}
		case arrayRankFeature:
			if len(want) != 1 || want[0] != int64(len(dims)) {
				return refuse(declares)
			}
		case arrayFlattenedSizeFeature:
			if len(want) != 1 || want[0] != size {
				return refuse(declares)
			}
		}
	}
	return ""
}

// constantIntegers reads the Integers a feature of owner states as a constant
// value: its own, or the one it takes from what it redefines. Reports false for
// a feature stating none, or one only evaluation settles.
func (ctx *Context) constantIntegers(member, owner *symbols.Symbol) ([]int64, bool) {
	value := ctx.extractDefaultValue(member)
	if value == nil {
		value, _ = ctx.redefinedDefault(member, owner)
	}
	if value == nil {
		return nil, false
	}
	var elements []ast.Node
	switch n := value.(type) {
	case *ast.NullExpr:
	case *ast.SequenceExpr:
		elements = n.Elements
	default:
		elements = []ast.Node{value}
	}
	out := make([]int64, 0, len(elements))
	for _, e := range elements {
		c, ok := ctx.model.Eval(e)
		if !ok {
			return nil, false
		}
		n, ok := c.WholeNumber()
		if !ok {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// elementsConform judges the elements (a vector quantity's magnitudes) against each
// effective feature that is or redefines Array's `elements`: count and element type.
func (ctx *Context) elementsConform(scope *symbols.Scope, value Value, declared *symbols.Symbol, how admission) (bool, string, error) {
	elements := structuredElements(value)
	refuse := func(feat EffectiveFeature, mult, got string) string {
		declares := feat.Name
		if feat.Type != nil {
			declares += " : " + symbolText(feat.Type)
		}
		return fmt.Sprintf("cannot write %s (%s) to a feature typed by %s: it declares %s%s, got %s",
			FormatValue(value), describeValue(value), symbolText(declared), declares, mult, got)
	}
	for _, feat := range ctx.FeaturesOf(declared) {
		if base, ok := ctx.arrayFeatureOf(feat.Symbol); !ok || base != arrayElementsFeature {
			continue
		}
		if feat.Multiplicity.CountViolation(int64(len(elements))) != "" {
			return false, refuse(feat, feat.Multiplicity.Text(), fmt.Sprintf("%d element(s)", len(elements))), nil
		}
		if feat.Type == nil {
			continue
		}
		for _, elem := range elements {
			ok, refusal, err := ctx.valueConforms(scope, &elem, feat.Type, how)
			if err != nil || !ok {
				if refusal == "" {
					refusal = refuse(feat, "", fmt.Sprintf("element %s (%s)", FormatValue(elem), describeValue(elem)))
				}
				return false, refusal, err
			}
		}
	}
	return true, "", nil
}

// structuredDimensions is the dimensions of an array or tensor, or the one of a vector.
func structuredDimensions(value Value) []int64 {
	switch value.Kind {
	case ValArray:
		return value.Array().Dimensions
	case ValVector:
		return []int64{int64(value.Vector().Dimension())}
	case ValVectorQuantity:
		return []int64{int64(value.VectorQuantity().Dimension())}
	case ValTensorQuantity:
		return value.TensorQuantity().Dimensions
	}
	return nil
}

// structuredElements is the row-major elements of an array or vector, or the
// magnitudes of a vector or tensor quantity's components.
func structuredElements(value Value) []Value {
	switch value.Kind {
	case ValArray:
		return value.Array().Elements
	case ValVector:
		return constValues(value.Vector().Elements)
	case ValVectorQuantity:
		return constValues(value.VectorQuantity().Num)
	case ValTensorQuantity:
		return constValues(value.TensorQuantity().Num)
	}
	return nil
}

// equalInt64s reports whether two lists hold the same numbers in order.
func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// quantityConforms judges a written quantity: a scalar target by the lattice, a
// target declaring a quantity value type by the dimension that type's mRef
// fixes. A target fixing no dimension, and a unit fixing none, are not judged.
func (ctx *Context) quantityConforms(value Value, declared *symbols.Symbol) (bool, string, error) {
	if prim := ctx.model.PrimTypeOf(declared); prim != semantics.PrimUnknown {
		return semantics.PrimConforms(semantics.PrimRational, prim), "", nil
	}
	want, ok := ctx.model.DimensionOfType(declared)
	if !ok || value.Quantity() == nil {
		return true, "", nil
	}
	got, ok := ctx.model.DimensionOfUnit(value.Quantity().Unit.Term)
	if !ok || want.Term.Commensurable(got.Term) {
		return true, "", nil
	}
	return false, fmt.Sprintf("cannot write %s (%s) to a feature typed by %s (%s)",
		FormatValue(value), dimensionText(got), symbolText(declared), dimensionText(want)), nil
}

// dimensionText names a dimension as a message about a mismatch reads it.
func dimensionText(d semantics.Dimension) string {
	if d.Term.Dimensionless() {
		return "dimensionless"
	}
	return "dimension " + d.String()
}
