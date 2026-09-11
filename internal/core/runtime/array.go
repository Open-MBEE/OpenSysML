package runtime

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Array is a Collections::Array: its `dimensions` and its `elements` flattened in
// row-major order. `rank` and `flattenedSize` derive from the dimensions.
type Array struct {
	Dimensions []int64
	Elements   []Value
	// Object is the Collections::Array object the value was read from, which keeps
	// answering the members a specialization adds; 0 for an array made by value.
	Object int64
}

// NewArrayValue wraps an array over its row-major elements; arrayOf checks the
// invariant `flattenedSize == size(elements)`.
func NewArrayValue(dimensions []int64, elements []Value) Value {
	return Value{Kind: ValArray, ref: &Array{Dimensions: dimensions, Elements: elements}}
}

// Rank is the number of dimensions.
func (a *Array) Rank() int {
	return len(a.Dimensions)
}

// FlattenedSize is the product of the dimensions, one for an array of none.
func (a *Array) FlattenedSize() int64 {
	size, _ := flattenedSize(a.Dimensions)
	return size
}

// flattenedSize is the product of positive dimensions, one for the empty list;
// false when the product does not fit an Integer.
func flattenedSize(dimensions []int64) (int64, bool) {
	size := int64(1)
	for _, d := range dimensions {
		if d > 0 && size > math.MaxInt64/d {
			return 0, false
		}
		size *= d
	}
	return size, true
}

// at is the element at one one-based index per dimension, row-major (the last
// index varies fastest) as CollectionFunctions::'array#' computes it.
func (a *Array) at(op string, indexes []int64) (Value, error) {
	if len(indexes) != a.Rank() {
		return Value{}, fmt.Errorf(
			"%w: %s: %d indexes address an array of rank %d (indexes: Positive[n], n = arr.rank)",
			ErrMultiplicityViolation, op, len(indexes), a.Rank(),
		)
	}
	offset := int64(0)
	for i, index := range indexes {
		if index < 1 || index > a.Dimensions[i] {
			return Value{}, fmt.Errorf(
				"%w: %s: index %d is %d, dimension %d has 1..%d",
				ErrIndexOutOfRange, op, i+1, index, i+1, a.Dimensions[i],
			)
		}
		next, ok := rowMajorOffset(offset, a.Dimensions[i], index-1)
		if !ok {
			return Value{}, fmt.Errorf(
				"%w: %s: the row-major offset of %s in dimensions %s exceeds the Integer range",
				semantics.ErrArithmeticOverflow, op, FormatValue(intSequence(indexes)), FormatValue(intSequence(a.Dimensions)),
			)
		}
		offset = next
	}
	if offset >= int64(len(a.Elements)) {
		return Value{}, fmt.Errorf(
			"%w: %s: %s addresses row-major offset %d, the array holds %d element(s)",
			ErrIndexOutOfRange, op, FormatValue(intSequence(indexes)), offset, len(a.Elements),
		)
	}
	return a.Elements[offset], nil
}

// rowMajorOffset is offset*dimension + index, false when it does not fit an Integer.
func rowMajorOffset(offset, dimension, index int64) (int64, bool) {
	if dimension > 0 && offset > math.MaxInt64/dimension {
		return 0, false
	}
	scaled := offset * dimension
	if index > math.MaxInt64-scaled {
		return 0, false
	}
	return scaled + index, true
}

// Format renders `Array(2, 2)[1, 2, 3, 4]` with each element rendered by
// element; an array of rank 0 is `Array()[7]`.
func (a *Array) Format(element func(Value) string) string {
	dims := make([]string, len(a.Dimensions))
	for i, d := range a.Dimensions {
		dims[i] = fmt.Sprintf("%d", d)
	}
	parts := make([]string, len(a.Elements))
	for i, e := range a.Elements {
		parts[i] = element(e)
	}
	return "Array(" + strings.Join(dims, ", ") + ")[" + strings.Join(parts, ", ") + "]"
}

// Vector is a VectorValues::NumericalVectorValue: the numbers of an Array of one
// dimension, its `dimension`.
type Vector struct {
	Elements []semantics.Value
	// Object is the NumericalVectorValue object the value was read from, which keeps
	// answering the members a specialization adds; 0 for a vector made by value.
	Object int64
}

// NewVectorValue wraps the vector of the given numeric components.
func NewVectorValue(elements []semantics.Value) Value {
	return Value{Kind: ValVector, ref: &Vector{Elements: elements}}
}

// Dimension is the number of components.
func (v *Vector) Dimension() int {
	return len(v.Elements)
}

// format renders `⟨1, 2, 3⟩`, distinct from the sequence `[1, 2, 3]`.
func (v *Vector) format(element func(semantics.Value) string) string {
	parts := make([]string, len(v.Elements))
	for i, e := range v.Elements {
		parts[i] = element(e)
	}
	return "⟨" + strings.Join(parts, ", ") + "⟩"
}

// VectorQuantity is a Quantities::VectorQuantityValue: the numbers `num` and the
// unit each axis of its measurement reference (`mRef.mRefs`) is expressed in.
type VectorQuantity struct {
	Num   []semantics.Value
	Units []Unit
	// Frame is the coordinate frame the vector was written over (`(1, 2, 3) [datum]`),
	// whose axes Units are; nil for one written over a scalar unit.
	Frame *CoordinateFrame
}

// NewVectorQuantityValue wraps a vector quantity; num and units are the same length.
func NewVectorQuantityValue(num []semantics.Value, units []Unit) Value {
	return Value{Kind: ValVectorQuantity, ref: &VectorQuantity{Num: num, Units: units}}
}

// NewFramedVectorQuantityValue wraps a vector quantity over a coordinate frame,
// one component per axis.
func NewFramedVectorQuantityValue(num []semantics.Value, frame *CoordinateFrame) Value {
	return Value{Kind: ValVectorQuantity, ref: &VectorQuantity{Num: num, Units: frame.Axes, Frame: frame}}
}

// Dimension is the number of components.
func (vq *VectorQuantity) Dimension() int {
	return len(vq.Num)
}

// component is the scalar quantity at one axis.
func (vq *VectorQuantity) component(i int) *Quantity {
	return &Quantity{Num: vq.Num[i], Unit: vq.Units[i]}
}

// sharedUnit is the one unit every axis measures in, spelt as the first axis
// spells it (`m` for axes in `m` and `SI::m`), when the axes' references are equal.
func (vq *VectorQuantity) sharedUnit() (Unit, bool) {
	if len(vq.Units) == 0 {
		return Unit{}, false
	}
	first := &MeasurementRef{Unit: vq.Units[0]}
	for _, u := range vq.Units[1:] {
		if !first.equal(&MeasurementRef{Unit: u}) {
			return Unit{}, false
		}
	}
	return vq.Units[0], true
}

// format renders `⟨1.0, 2.0⟩ [m]`, `⟨1.0, 2.0, 3.0⟩ [datum]` over a frame, or
// `⟨1.0 [m], 2.0 [rad]⟩` when the axes differ and no frame names them.
func (vq *VectorQuantity) format(element func(semantics.Value) string) string {
	parts := make([]string, len(vq.Num))
	if vq.Frame != nil {
		for i, n := range vq.Num {
			parts[i] = element(n)
		}
		return "⟨" + strings.Join(parts, ", ") + "⟩ [" + vq.Frame.Name() + "]"
	}
	if unit, ok := vq.sharedUnit(); ok {
		for i, n := range vq.Num {
			parts[i] = element(n)
		}
		return "⟨" + strings.Join(parts, ", ") + "⟩ [" + unit.String() + "]"
	}
	for i := range vq.Num {
		parts[i] = vq.component(i).TextWithMagnitude(element(vq.Num[i]))
	}
	return "⟨" + strings.Join(parts, ", ") + "⟩"
}

// Features of Collections::Array and its vector specializations answered from
// the value itself.
const (
	arrayDimensionsFeature    = "dimensions"
	arrayRankFeature          = "rank"
	arrayFlattenedSizeFeature = "flattenedSize"
	arrayElementsFeature      = "elements"
	vectorDimensionFeature    = "dimension"
	vectorQuantityNumFeature  = "num"
	vectorQuantityMRefFeature = "mRef"
	tensorOrderFeature        = "order"
)

// arrayFeatureNames are the features of Collections::Array a structured value
// answers from itself.
var arrayFeatureNames = []string{arrayDimensionsFeature, arrayRankFeature, arrayFlattenedSizeFeature, arrayElementsFeature}

// vectorQuantityAliases are the TensorQuantityValue features that redefine an
// Array feature (`num :>> elements`, `order :>> rank`), by the feature redefined.
var vectorQuantityAliases = map[string]string{
	vectorQuantityNumFeature: arrayElementsFeature,
	tensorOrderFeature:       arrayRankFeature,
}

// Library types the structured kinds are values of.
const (
	arrayTypeFQN                = "Collections::Array"
	vectorTypeFQN               = "VectorValues::VectorValue"
	numericalVectorTypeFQN      = "VectorValues::NumericalVectorValue"
	cartesianVectorTypeFQN      = "VectorValues::CartesianVectorValue"
	cartesianThreeVectorTypeFQN = "VectorValues::CartesianThreeVectorValue"
	vectorQuantityTypeFQN       = "Quantities::VectorQuantityValue"
	scalarValueTypeFQN          = "ScalarValues::ScalarValue"
	positiveTypeFQN             = "ScalarValues::Positive"
	scalarValuesPackageFQN      = "ScalarValues"
)

// structuredFeature reads a library feature of an array, vector, vector, tensor
// or scalar quantity, or measurement reference; the second result is
// false for another name. A sequence it answers is charged to the element budget.
func (ctx *Context) structuredFeature(val Value, name string) (Value, bool, error) {
	switch val.Kind {
	case ValArray:
		a := val.Array()
		switch name {
		case arrayDimensionsFeature:
			return ctx.answerSequence(integerValues(a.Dimensions))
		case arrayRankFeature:
			return integerValue(int64(a.Rank())), true, nil
		case arrayFlattenedSizeFeature:
			return integerValue(a.FlattenedSize()), true, nil
		case arrayElementsFeature:
			return ctx.answerSequence(a.Elements)
		}
	case ValVector:
		return ctx.oneDimensionalFeature(name, val.Vector().Elements, nil)
	case ValVectorQuantity:
		vq := val.VectorQuantity()
		if name == vectorQuantityMRefFeature {
			ref, err := vectorQuantityMRef(vq)
			return ref, true, err
		}
		return ctx.oneDimensionalFeature(name, vq.Num, vectorQuantityAliases)
	case ValQuantity:
		return ctx.quantityFeature(val, name)
	case ValMeasurementRef:
		return ctx.measurementRefFeature(val, name)
	case ValTensorQuantity:
		return ctx.tensorQuantityFeature(val, name)
	case ValCoordinateFrame:
		return ctx.frameFeature(val, name)
	case ValCoordinateTransformation:
		return ctx.transformationFeature(val, name)
	}
	return Value{}, false, nil
}

// oneDimensionalFeature answers a vector's Array features and `dimension` from
// its components; aliases maps the type's further names onto the Array feature.
func (ctx *Context) oneDimensionalFeature(name string, components []semantics.Value, aliases map[string]string) (Value, bool, error) {
	if base, ok := aliases[name]; ok {
		name = base
	}
	switch name {
	case vectorDimensionFeature, arrayFlattenedSizeFeature:
		return integerValue(int64(len(components))), true, nil
	case arrayDimensionsFeature:
		return ctx.answerSequence(integerValues([]int64{int64(len(components))}))
	case arrayRankFeature:
		return integerValue(1), true, nil
	case arrayElementsFeature:
		return ctx.answerSequence(constValues(components))
	}
	return Value{}, false, nil
}

// arrayFeatureOf is the Collections::Array feature a member is or transitively
// redefines (`dimension :>> dimensions`); false for a specialization's own member.
func (ctx *Context) arrayFeatureOf(member *symbols.Symbol) (string, bool) {
	names := ctx.arrayFeatureSymbols()
	if name, ok := names[member]; ok {
		return name, true
	}
	for _, redefined := range ctx.model.semantics.AllRedefinedFeatures(member) {
		if name, ok := names[redefined]; ok {
			return name, true
		}
	}
	return "", false
}

// arrayFeatureNamed is the Collections::Array feature the member of typ named
// name is or redefines, and that member; false for another member, or none.
func (ctx *Context) arrayFeatureNamed(typ *symbols.Symbol, name string) (string, *symbols.Symbol, bool) {
	member, ok := ctx.model.semantics.LookupMember(typ, name)
	if !ok || member == nil {
		return "", nil, false
	}
	base, ok := ctx.arrayFeatureOf(member)
	return base, member, ok
}

// structuredMember answers an Array feature as the member of owner declares it:
// `dimension[0..1] :>> dimensions` is the one dimension, not the sequence.
func (ctx *Context) structuredMember(val Value, base string, member, owner *symbols.Symbol) (Value, error) {
	if base == arrayDimensionsFeature {
		if upper := ctx.featureMultiplicity(member, owner).Upper; upper.Known && !upper.Infinite && upper.Value == 1 {
			dims := structuredDimensions(val)
			if len(dims) == 0 {
				return nullValue(), nil
			}
			return integerValue(dims[0]), nil
		}
	}
	answer, ok, err := ctx.structuredFeature(val, base)
	if err != nil {
		return Value{}, err
	}
	if !ok {
		return Value{}, fmt.Errorf("%w: %s has no feature %s", ErrTypeMismatch, describeValue(val), base)
	}
	return answer, nil
}

// arrayFeatureSymbols maps each declaration of a Collections::Array feature (and
// those it redefines, `OrderedCollection::elements` included) to its name. Memoized.
func (ctx *Context) arrayFeatureSymbols() map[*symbols.Symbol]string {
	if ctx.model.arrayFeatures != nil {
		return ctx.model.arrayFeatures
	}
	names := make(map[*symbols.Symbol]string)
	if arraySym := ctx.librarySymbol(arrayTypeFQN); arraySym != nil {
		for _, name := range arrayFeatureNames {
			feat, ok := ctx.model.semantics.LookupMember(arraySym, name)
			if !ok || feat == nil {
				continue
			}
			names[feat] = name
			for _, redefined := range ctx.model.semantics.AllRedefinedFeatures(feat) {
				names[redefined] = name
			}
		}
	}
	ctx.model.arrayFeatures = names
	return names
}

// answerSequence is a feature answered as a fresh, charged sequence of elements.
func (ctx *Context) answerSequence(elements []Value) (Value, bool, error) {
	seq, err := ctx.newSequence(elements)
	return seq, true, err
}

// backingObject is the object an array or vector was read from; 0 for a value
// made by value, and for the other kinds.
func backingObject(value Value) int64 {
	switch value.Kind {
	case ValArray:
		return value.Array().Object
	case ValVector:
		return value.Vector().Object
	case ValCoordinateFrame:
		return value.CoordinateFrame().Object
	case ValCoordinateTransformation:
		return value.CoordinateTransformation().Object
	}
	return 0
}

// keptObject is the object a value keeps: the one an array or vector was read
// from, or a tensor quantity built over; 0 for none.
func keptObject(value Value) int64 {
	if value.Kind == ValTensorQuantity {
		return value.TensorQuantity().MRef
	}
	return backingObject(value)
}

// structuredObject is the live object an array or vector was read from, if any.
func (ctx *Context) structuredObject(value Value) (*Instance, bool) {
	inst, ok := ctx.instances[backingObject(value)]
	return inst, ok
}

// structuredValueType is the most specific library type a structured value is
// of: the object it was read from or an Array, a Cartesian (three-)vector, a
// vector or tensor quantity.
func (ctx *Context) structuredValueType(value Value) (*symbols.Symbol, error) {
	if inst, ok := ctx.structuredObject(value); ok {
		return ctx.objectType(inst), nil
	}
	var fqn string
	switch value.Kind {
	case ValArray:
		fqn = arrayTypeFQN
	case ValVector:
		// Every number is a Real, so a numerical vector is a Cartesian one.
		fqn = cartesianVectorTypeFQN
		if value.Vector().Dimension() == 3 {
			fqn = cartesianThreeVectorTypeFQN
		}
	case ValVectorQuantity:
		fqn = vectorQuantityTypeFQN
	case ValTensorQuantity:
		fqn = tensorQuantityTypeFQN
	default:
		return nil, fmt.Errorf("%w: %s", ErrUndeterminedValueType, value.Kind)
	}
	return ctx.loadedLibraryType(fqn)
}

// structuredBaseType is the type a structured value is of whatever its shape (Array,
// NumericalVectorValue, VectorQuantityValue, TensorQuantityValue, or the type of the
// object it was read from).
func (ctx *Context) structuredBaseType(value Value) (*symbols.Symbol, error) {
	if inst, ok := ctx.structuredObject(value); ok {
		return ctx.objectType(inst), nil
	}
	var fqn string
	switch value.Kind {
	case ValArray:
		fqn = arrayTypeFQN
	case ValVector:
		fqn = numericalVectorTypeFQN
	case ValVectorQuantity:
		fqn = vectorQuantityTypeFQN
	case ValTensorQuantity:
		fqn = tensorQuantityTypeFQN
	default:
		return nil, fmt.Errorf("%w: %s", ErrUndeterminedValueType, value.Kind)
	}
	return ctx.loadedLibraryType(fqn)
}

// loadedLibraryType is the library type of the given name, an error if the
// library declaring it is not loaded.
func (ctx *Context) loadedLibraryType(fqn string) (*symbols.Symbol, error) {
	sym := ctx.librarySymbol(fqn)
	if sym == nil {
		return nil, fmt.Errorf("%w: type %q is not loaded", ErrUndeterminedValueType, fqn)
	}
	return sym, nil
}

// arrayOfObject reads an object typed by Collections::Array as the Array value its
// `dimensions` and `elements` shape — the vector, for a NumericalVectorValue of
// one dimension; one stating neither stays an object.
func (ctx *Context) arrayOfObject(inst *Instance) (Value, bool, error) {
	arraySym := ctx.librarySymbol(arrayTypeFQN)
	if arraySym == nil || inst == nil || inst.Type == nil {
		return Value{}, false, nil
	}
	if !ctx.model.semantics.Conforms(ctx.objectType(inst), arraySym) || !ctx.shapeHoldsValue(inst.Type) {
		return Value{}, false, nil
	}
	dims, dimsStated, err := ctx.objectArrayFeature(inst, arrayDimensionsFeature)
	if err != nil {
		return Value{}, true, err
	}
	elements, elementsStated, err := ctx.objectArrayFeature(inst, arrayElementsFeature)
	if err != nil {
		return Value{}, true, err
	}
	if !dimsStated && !elementsStated {
		return Value{}, false, nil
	}
	op := arrayTypeFQN + " " + symbolText(inst.Type)
	dimensions, err := indexList(op+" dimensions", dims)
	if err != nil {
		return Value{}, true, err
	}
	if !dimsStated {
		if fixed, ok := ctx.model.semantics.FixedDimensions(ctx.objectType(inst)); ok {
			dimensions = fixed
		}
	}
	val, err := arrayOf(op, dimensions, elements)
	if err != nil {
		return Value{}, true, err
	}
	val.Array().Object = inst.ID
	return ctx.vectorOfObject(inst, val)
}

// vectorOfObject is the vector a NumericalVectorValue object of one dimension is;
// a rank-0 one stays the Array, as does a VectorQuantityValue (no unit to take).
func (ctx *Context) vectorOfObject(inst *Instance, array Value) (Value, bool, error) {
	vectorSym := ctx.librarySymbol(numericalVectorTypeFQN)
	typ := ctx.objectType(inst)
	if vectorSym == nil || !ctx.model.semantics.Conforms(typ, vectorSym) {
		return array, true, nil
	}
	a := array.Array()
	op := numericalVectorTypeFQN + " " + symbolText(inst.Type)
	if a.Rank() > 1 {
		return Value{}, true, fmt.Errorf(
			"%w: %s: %d dimensions, NumericalVectorValue declares dimension[0..1]",
			ErrMultiplicityViolation, op, a.Rank(),
		)
	}
	quantitySym := ctx.librarySymbol(vectorQuantityTypeFQN)
	if a.Rank() == 0 || (quantitySym != nil && ctx.model.semantics.Conforms(typ, quantitySym)) {
		return array, true, nil
	}
	components := make([]semantics.Value, len(a.Elements))
	for i, elem := range a.Elements {
		if elem.Kind != ValConst || !elem.Const.IsNumeric() {
			return Value{}, true, fmt.Errorf(
				"%w: %s: element %d is %s, NumericalVectorValue declares elements : NumericalValue",
				ErrTypeMismatch, op, i+1, describeValue(elem),
			)
		}
		components[i] = elem.Const
	}
	val, err := ctx.vectorValue(components)
	if err != nil {
		return Value{}, true, err
	}
	val.Vector().Object = inst.ID
	return val, true, nil
}

// objectArrayFeature is the values an object holds under a Collections::Array
// feature by whichever name its type declares it, and whether it is stated.
func (ctx *Context) objectArrayFeature(inst *Instance, name string) ([]Value, bool, error) {
	for _, feat := range ctx.FeaturesOf(inst.Type) {
		if base, ok := ctx.arrayFeatureOf(feat.Symbol); !ok || base != name {
			continue
		}
		elements, stated, err := ctx.objectFeatureElements(inst, feat.Name)
		if err != nil || stated {
			return elements, stated, err
		}
	}
	return nil, false, nil
}

// objectType is the type an object instantiates: the type of the usage it
// occurs as, else the definition itself.
func (ctx *Context) objectType(inst *Instance) *symbols.Symbol {
	if typ := ctx.extractType(inst.Type); typ != nil {
		return typ
	}
	return inst.Type
}

// declaredArrayValue reads a valueless usage typed by Collections::Array as the
// Array its own dimensions and elements shape. Reports whether the usage is one.
func (ctx *Context) declaredArrayValue(sym *symbols.Symbol) (Value, bool, error) {
	arraySym := ctx.librarySymbol(arrayTypeFQN)
	typ := ctx.extractType(sym)
	if arraySym == nil || typ == nil || !ctx.model.semantics.Conforms(typ, arraySym) || !ctx.namesOneObject(sym) {
		return Value{}, false, nil
	}
	inst, err := ctx.occurrenceOf(sym)
	if err != nil {
		return Value{}, true, fmt.Errorf("usage %s: %w", symbolText(sym), err)
	}
	if val, ok, err := ctx.referenceValueOfObject(inst); ok {
		return val, true, err
	}
	return ctx.arrayOfObject(inst)
}

// objectFeatureElements is the values an object's feature holds, materialized on
// demand, and whether the feature is stated: bound to a value (an empty one too),
// written by a run, or derived through a binding, rather than merely inherited.
func (ctx *Context) objectFeatureElements(inst *Instance, name string) ([]Value, bool, error) {
	if _, ok := inst.FeatureValues[name]; !ok {
		return nil, false, nil
	}
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		return nil, true, err
	}
	val, err := fv.ReadValue(name)
	if err != nil {
		return nil, true, err
	}
	stated := fv.Feature.DefaultValue != nil || fv.Written || fv.BindingDerived
	return elementsOf(val), stated, nil
}

// objectValue is what a name denoting an object evaluates to: a unit declaration's
// reference, the frame, scale or transformation a MeasurementReferences object is,
// the Array a shaped Collections::Array object is, else the object itself.
func (ctx *Context) objectValue(inst *Instance) (Value, error) {
	if val, ok, err := ctx.unitObjectValue(inst); ok {
		return val, err
	}
	if val, ok, err := ctx.referenceValueOfObject(inst); ok {
		return val, err
	}
	if val, ok, err := ctx.arrayOfObject(inst); ok {
		return val, err
	}
	return Value{Kind: ValInstance, Instance: inst.ID}, nil
}

// keyBytes serializes a valueKey, so values valueEqual holds equal serialize alike.
func keyBytes(k valueKey) []byte {
	flag := func(b bool) uint64 {
		if b {
			return 1
		}
		return 0
	}
	// #nosec G115 -- the two's-complement bits are what the hash wants.
	buf := binary.LittleEndian.AppendUint64(nil, uint64(k.kind))
	// #nosec G115 -- see above.
	buf = binary.LittleEndian.AppendUint64(buf, uint64(k.intVal))
	buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(k.realVal))
	buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(k.imagVal))
	buf = binary.LittleEndian.AppendUint64(buf, flag(k.boolVal)<<1|flag(k.infVal))
	buf = binary.LittleEndian.AppendUint64(buf, k.colHash)
	return append(buf, k.strVal...)
}

// valueHash is the content hash of a value's key, for the key of a value holding it:
// a quantity hashes by its base magnitude and dimension, as its equality compares.
func valueHash(v Value) uint64 {
	h := fnv.New64a()
	// #nosec G104 -- hash.Hash.Write is documented never to return an error.
	h.Write(keyBytes(valueKeyFunc(v)))
	return h.Sum64()
}

// structuredKey is the content hash a structured value's valueKey carries.
func (ctx *Context) structuredKey(v Value) uint64 {
	h := fnv.New64a()
	write := func(k valueKey) {
		// #nosec G104 -- hash.Hash.Write is documented never to return an error.
		h.Write(keyBytes(k))
	}
	switch v.Kind {
	case ValArray:
		for _, d := range v.Array().Dimensions {
			write(valueKeyFunc(integerValue(d)))
		}
		for _, e := range v.Array().Elements {
			write(ctx.valueKey(e))
		}
	case ValVector:
		for _, e := range v.Vector().Elements {
			write(valueKeyFunc(Value{Kind: ValConst, Const: e}))
		}
	case ValVectorQuantity:
		vq := v.VectorQuantity()
		for i := range vq.Num {
			write(ctx.valueKey(NewQuantityValue(vq.component(i))))
		}
	case ValTensorQuantity:
		tq := v.TensorQuantity()
		for _, d := range tq.Dimensions {
			write(valueKeyFunc(integerValue(d)))
		}
		for _, component := range tq.components() {
			write(ctx.valueKey(component))
		}
	}
	return h.Sum64()
}

// integerValues wraps counts as Integer values.
func integerValues(ns []int64) []Value {
	out := make([]Value, len(ns))
	for i, n := range ns {
		out[i] = integerValue(n)
	}
	return out
}

// intSequence is the uncharged sequence of Integer constants a diagnostic renders.
func intSequence(ns []int64) Value {
	return sequenceOf(integerValues(ns))
}

// constValue wraps a numeric constant as a runtime value.
func constValue(c semantics.Value) Value {
	return Value{Kind: ValConst, Const: c}
}

// constValues wraps numeric constants as runtime values.
func constValues(consts []semantics.Value) []Value {
	out := make([]Value, len(consts))
	for i, c := range consts {
		out[i] = constValue(c)
	}
	return out
}

// arrayOf builds an Array, holding the library's invariants: every dimension is
// Positive and the elements number the flattened size.
func arrayOf(op string, dimensions []int64, elements []Value) (Value, error) {
	for i, d := range dimensions {
		if d < 1 {
			return Value{}, fmt.Errorf(
				"%w: %s: dimension %d is %d, Collections::Array declares dimensions: Positive",
				ErrMultiplicityViolation, op, i+1, d,
			)
		}
	}
	want, ok := flattenedSize(dimensions)
	if !ok {
		return Value{}, fmt.Errorf(
			"%w: %s: flattenedSize of dimensions %s exceeds the Integer range",
			semantics.ErrArithmeticOverflow, op, FormatValue(intSequence(dimensions)),
		)
	}
	if want != int64(len(elements)) {
		return Value{}, fmt.Errorf(
			"%w: %s: %d elements do not fill dimensions %s (flattenedSize %d)",
			ErrMultiplicityViolation, op, len(elements), FormatValue(intSequence(dimensions)), want,
		)
	}
	return NewArrayValue(dimensions, elements), nil
}

// indexList reads the Positive indexes of an indexing operation.
func indexList(op string, indexes []Value) ([]int64, error) {
	out := make([]int64, len(indexes))
	for i, index := range indexes {
		n, err := indexOf(op, index)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}
