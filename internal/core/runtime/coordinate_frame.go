package runtime

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CoordinateFrame is a VectorMeasurementReference held as a value: the axes of a
// coordinate frame, or the one axis a measurement scale is, with the placement
// relating it to another reference where the model states one.
type CoordinateFrame struct {
	Decl           *symbols.Symbol           // the usage declaring the frame; nil for one composed by arithmetic
	Type           *symbols.Symbol           // the type the frame is a value of
	Dimensions     []int64                   // `dimensions`: (3) for a 3-D frame, () for a scale
	Axes           []Unit                    // `mRefs`, one per element the dimensions shape
	Transformation *CoordinateTransformation // `transformation`, whose target is this frame; nil for none
	Scale          *MeasurementScale         // what a MeasurementScale adds; nil for a frame
	Object         int64                     // the object the frame was read from; 0 for one composed
	Text           string                    // the frame as written: `spatialCF`, `spatialCF / s`
}

// MeasurementScale is what a scale states beyond its one axis: the unit a
// magnitude on it is in, and the point mapping it onto another reference.
type MeasurementScale struct {
	Unit    Unit
	Mapping *QuantityValueMapping
}

// QuantityValueMapping is one point given on the scale and on the reference it
// is defined against, as `quantityValueMapping` states it.
type QuantityValueMapping struct {
	Mapped    Quantity // the point on the scale, in the scale's unit
	Reference Quantity // the same point on the reference
}

// CoordinateTransformation is a CoordinateTransformation held as a value: the
// placement of the target in the source by at most one of Placement, Sequence
// and Affine; a subtype of no library shape holds none and cannot be applied.
type CoordinateTransformation struct {
	Decl      *symbols.Symbol  // the usage declaring the transformation; nil for a constructed one
	Type      *symbols.Symbol  // the type the transformation is a value of
	Source    *CoordinateFrame // `source`; nil when the model states none
	Target    *CoordinateFrame // `target`; nil for a frame's own transformation, whose target it is
	Placement *FramePlacement
	Sequence  []FrameStep
	Affine    *AffineTransformation3d
	Object    int64
}

// FramePlacement is a CoordinateFramePlacement: the target frame's origin as a
// vector in the source frame, and its basis directions there; none for the
// source's own orientation.
type FramePlacement struct {
	Origin          Value   // a vector quantity, or a scalar quantity for a one-dimensional frame
	BasisDirections []Value // vector quantities, one per target axis
}

// FrameStep is one TranslationOrRotation of a TranslationRotationSequence.
type FrameStep struct {
	Translation *Value    // Translation::translationVector, a vector quantity
	Axis        *Value    // Rotation::axisDirection, a vector quantity
	Angle       *Quantity // Rotation::angle, an angular measure
	Intrinsic   bool      // Rotation::isIntrinsic
	Object      int64
}

// AffineTransformation3d is an AffineTransformationMatrix3d: the 3x3 rotation
// in row-major order and the translation, both bare numbers.
type AffineTransformation3d struct {
	Rotation    [9]float64
	Translation [3]float64
}

// NewCoordinateFrameValue wraps a frame as a value.
func NewCoordinateFrameValue(frame *CoordinateFrame) Value {
	return Value{Kind: ValCoordinateFrame, ref: frame}
}

// NewCoordinateTransformationValue wraps a transformation as a value.
func NewCoordinateTransformationValue(t *CoordinateTransformation) Value {
	return Value{Kind: ValCoordinateTransformation, ref: t}
}

// IsScale reports whether the frame is a measurement scale.
func (f *CoordinateFrame) IsScale() bool { return f != nil && f.Scale != nil }

// FlattenedSize is the number of axes the dimensions shape: their product, 1 for
// a scalar reference's empty dimensions; false when it does not fit an Integer.
func (f *CoordinateFrame) FlattenedSize() (int64, bool) {
	return flattenedSize(f.Dimensions)
}

// flattenedSizeError is the typed refusal of dimensions whose product overflows.
func (f *CoordinateFrame) flattenedSizeError(what string) error {
	return fmt.Errorf("%w: %s: flattenedSize of dimensions %s exceeds the Integer range",
		semantics.ErrArithmeticOverflow, what, FormatValue(intSequence(f.Dimensions)))
}

// Name is how the frame is referred to in diagnostics: its declared name, else
// how it was composed.
func (f *CoordinateFrame) Name() string {
	if f == nil {
		return unknownText
	}
	if f.Text != "" {
		return f.Text
	}
	if f.Decl != nil {
		return symbolText(f.Decl)
	}
	return "a coordinate frame"
}

// String renders the frame as its name over its axes: `spatialCF [m, m, m]`; a
// scale over the unit its magnitudes are in: `°C_abs [°C]`.
func (f *CoordinateFrame) String() string {
	if f == nil {
		return unknownText
	}
	if f.Scale != nil {
		return f.Name() + " [" + f.Scale.Unit.String() + "]"
	}
	axes := make([]string, len(f.Axes))
	for i, axis := range f.Axes {
		axes[i] = (&MeasurementRef{Unit: axis}).String()
	}
	return f.Name() + " [" + strings.Join(axes, ", ") + "]"
}

// axisRefs are the axes as measurement reference values, what `mRefs` answers.
func (f *CoordinateFrame) axisRefs() []Value {
	out := make([]Value, len(f.Axes))
	for i, axis := range f.Axes {
		out[i] = measurementRefOf(axis)
	}
	return out
}

// sameAxes holds when the two frames' axes are equal reference by reference.
func (f *CoordinateFrame) sameAxes(other *CoordinateFrame) bool {
	if len(f.Axes) != len(other.Axes) {
		return false
	}
	for i := range f.Axes {
		if !(&MeasurementRef{Unit: f.Axes[i]}).equal(&MeasurementRef{Unit: other.Axes[i]}) {
			return false
		}
	}
	return true
}

// equal holds for one identity: a declared frame is the object it was read from,
// a frame composed by `CoordinateFrame*` or `/` is its dimensions, axes, scale and
// transformation, whatever declaration it was later bound to.
func (f *CoordinateFrame) equal(other *CoordinateFrame) bool {
	if f == nil || other == nil {
		return f == other
	}
	if f.Object != 0 || other.Object != 0 {
		return f.Object == other.Object
	}
	if !equalInt64s(f.Dimensions, other.Dimensions) || !f.sameAxes(other) {
		return false
	}
	if (f.Scale == nil) != (other.Scale == nil) {
		return false
	}
	if f.Scale != nil && !f.Scale.equal(other.Scale) {
		return false
	}
	return f.Transformation.equal(other.Transformation)
}

// key identifies the frame the way equal compares it.
func (f *CoordinateFrame) key() string {
	if f == nil {
		return ""
	}
	if f.Object != 0 {
		return "obj:" + strconv.FormatInt(f.Object, 10)
	}
	var b strings.Builder
	b.WriteString("dims:")
	for _, d := range f.Dimensions {
		b.WriteString(strconv.FormatInt(d, 10) + ",")
	}
	b.WriteString("|axes:")
	for _, axis := range f.Axes {
		b.WriteString((&MeasurementRef{Unit: axis}).key() + ",")
	}
	if f.Scale != nil {
		b.WriteString("|scale:" + (&MeasurementRef{Unit: f.Scale.Unit}).key())
		if f.Scale.Mapping != nil {
			b.WriteString("|map:" + strconv.FormatUint(valueHash(NewQuantityValue(&f.Scale.Mapping.Mapped)), 10))
			b.WriteString("," + strconv.FormatUint(valueHash(NewQuantityValue(&f.Scale.Mapping.Reference)), 10))
		}
	}
	if f.Transformation != nil {
		b.WriteString("|xf:" + f.Transformation.key())
	}
	return b.String()
}

// equal holds for one unit and one mapping (or none on both sides).
func (s *MeasurementScale) equal(other *MeasurementScale) bool {
	if !(&MeasurementRef{Unit: s.Unit}).equal(&MeasurementRef{Unit: other.Unit}) {
		return false
	}
	if (s.Mapping == nil) != (other.Mapping == nil) {
		return false
	}
	if s.Mapping == nil {
		return true
	}
	return valueEqual(NewQuantityValue(&s.Mapping.Mapped), NewQuantityValue(&other.Mapping.Mapped)) &&
		valueEqual(NewQuantityValue(&s.Mapping.Reference), NewQuantityValue(&other.Mapping.Reference))
}

// Name is how the transformation is referred to in diagnostics.
func (t *CoordinateTransformation) Name() string {
	if t == nil {
		return unknownText
	}
	if t.Decl != nil {
		return symbolText(t.Decl)
	}
	if t.Type != nil {
		return "a " + symbolText(t.Type)
	}
	return "a coordinate transformation"
}

// sameDimensions is CoordinateTransformation's validSourceTargetDimensions: source
// and target state the same dimensions, so a vector over one is shaped for the other.
func (t *CoordinateTransformation) sameDimensions(name string) error {
	if equalInt64s(t.Source.Dimensions, t.Target.Dimensions) && len(t.Source.Axes) == len(t.Target.Axes) {
		return nil
	}
	return fmt.Errorf("%w: %s: %s relates %s of dimensions %s (%d axes) to %s of dimensions %s (%d axes); CoordinateTransformation asserts source.dimensions == target.dimensions",
		ErrMultiplicityViolation, name, t.Name(),
		t.Source.Name(), FormatValue(intSequence(t.Source.Dimensions)), len(t.Source.Axes),
		t.Target.Name(), FormatValue(intSequence(t.Target.Dimensions)), len(t.Target.Axes))
}

// String renders the transformation as its name over the frames it relates:
// `trs (datum → lbcf)`.
func (t *CoordinateTransformation) String() string {
	if t == nil {
		return unknownText
	}
	source, target := "?", "?"
	if t.Source != nil {
		source = t.Source.Name()
	}
	if t.Target != nil {
		target = t.Target.Name()
	}
	return t.Name() + " (" + source + " → " + target + ")"
}

// shapeName names the library shape the transformation has, for diagnostics.
func (t *CoordinateTransformation) shapeName() string {
	switch {
	case t.Placement != nil:
		return "CoordinateFramePlacement"
	case t.Sequence != nil:
		return "TranslationRotationSequence"
	case t.Affine != nil:
		return "AffineTransformationMatrix3d"
	}
	return "an unrecognized CoordinateTransformation"
}

// equal holds for equal source, target, shape and content (`lbcf.transformation == trs`);
// a transformation of no recognized shape has no content, so it equals only its own object.
func (t *CoordinateTransformation) equal(other *CoordinateTransformation) bool {
	if t == nil || other == nil {
		return t == other
	}
	if !t.Source.equal(other.Source) || !t.Target.equal(other.Target) {
		return false
	}
	if (t.Placement == nil) != (other.Placement == nil) || (t.Sequence == nil) != (other.Sequence == nil) ||
		(t.Affine == nil) != (other.Affine == nil) {
		return false
	}
	switch {
	case t.Placement != nil:
		if !valueEqual(t.Placement.Origin, other.Placement.Origin) ||
			len(t.Placement.BasisDirections) != len(other.Placement.BasisDirections) {
			return false
		}
		for i := range t.Placement.BasisDirections {
			if !valueEqual(t.Placement.BasisDirections[i], other.Placement.BasisDirections[i]) {
				return false
			}
		}
	case t.Sequence != nil:
		if len(t.Sequence) != len(other.Sequence) {
			return false
		}
		for i := range t.Sequence {
			if !t.Sequence[i].equal(other.Sequence[i]) {
				return false
			}
		}
	case t.Affine != nil:
		return *t.Affine == *other.Affine
	default:
		return t.Object == other.Object
	}
	return true
}

// equal holds for one step kind with equal parts.
func (s FrameStep) equal(other FrameStep) bool {
	if (s.Translation == nil) != (other.Translation == nil) || (s.Axis == nil) != (other.Axis == nil) {
		return false
	}
	if s.Translation != nil {
		return valueEqual(*s.Translation, *other.Translation)
	}
	return s.Intrinsic == other.Intrinsic && valueEqual(*s.Axis, *other.Axis) &&
		valueEqual(NewQuantityValue(s.Angle), NewQuantityValue(other.Angle))
}

// key identifies the transformation the way equal compares it.
func (t *CoordinateTransformation) key() string {
	if t == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(t.shapeName() + "|src:" + t.Source.key() + "|tgt:" + t.Target.key())
	switch {
	case t.Placement != nil:
		b.WriteString("|o:" + strconv.FormatUint(valueHash(t.Placement.Origin), 10))
		for _, dir := range t.Placement.BasisDirections {
			b.WriteString("|b:" + strconv.FormatUint(valueHash(dir), 10))
		}
	case t.Sequence != nil:
		for _, step := range t.Sequence {
			if step.Translation != nil {
				b.WriteString("|t:" + strconv.FormatUint(valueHash(*step.Translation), 10))
				continue
			}
			b.WriteString("|r:" + strconv.FormatUint(valueHash(*step.Axis), 10) +
				"," + strconv.FormatBool(step.Intrinsic) +
				"," + strconv.FormatUint(valueHash(NewQuantityValue(step.Angle)), 10))
		}
	case t.Affine != nil:
		b.WriteString(fmt.Sprintf("|m:%v%v", t.Affine.Rotation, t.Affine.Translation))
	default:
		b.WriteString("|obj:" + strconv.FormatInt(t.Object, 10))
	}
	return b.String()
}

// scalarFrame is the one-dimensional frame a scalar measurement reference is,
// what a placement's `source = K` names.
func scalarFrame(ref *MeasurementRef) *CoordinateFrame {
	return &CoordinateFrame{
		Decl: ref.Declaration(),
		Type: nil,
		Axes: []Unit{ref.Unit},
		Text: ref.String(),
	}
}

// frameOfReference is the frame a value names as a transformation's source or
// target: a frame itself, or the one-dimensional frame a scalar reference is.
func frameOfReference(what string, val Value) (*CoordinateFrame, error) {
	switch val.Kind {
	case ValCoordinateFrame:
		return val.CoordinateFrame(), nil
	case ValMeasurementRef:
		return scalarFrame(val.MeasurementRef()), nil
	}
	return nil, fmt.Errorf("%w: %s is %s, want a coordinate frame or a measurement reference",
		ErrTypeMismatch, what, describeValue(val))
}

// scaleAnchoredUnit is the unit a quantity `x [S]` on scale S carries: named by
// the scale's declaration, reducing to the scale itself so that it is
// commensurable with nothing but another point on the scale.
func scaleAnchoredUnit(decl *symbols.Symbol) Unit {
	name := unitSymbolName(decl)
	return Unit{
		Text:    name,
		Product: semantics.NamedUnitProduct(decl, name, false),
		Term:    semantics.UnitTerm{Scale: semantics.UnitScale(1), Factors: []semantics.UnitFactor{{Unit: decl, Exponent: 1}}},
	}
}
