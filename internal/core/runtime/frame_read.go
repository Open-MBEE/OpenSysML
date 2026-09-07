package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Library types a frame, scale or transformation is read against.
const (
	vectorMRefTypeFQN         = "MeasurementReferences::VectorMeasurementReference"
	measurementUnitTypeFQN    = "MeasurementReferences::MeasurementUnit"
	measurementScaleTypeFQN   = "MeasurementReferences::MeasurementScale"
	coordinateFrameTypeFQN    = "MeasurementReferences::CoordinateFrame"
	transformationTypeFQN     = "MeasurementReferences::CoordinateTransformation"
	placementTypeFQN          = "MeasurementReferences::CoordinateFramePlacement"
	rotationSequenceTypeFQN   = "MeasurementReferences::TranslationRotationSequence"
	translationTypeFQN        = "MeasurementReferences::Translation"
	rotationTypeFQN           = "MeasurementReferences::Rotation"
	affineMatrixTypeFQN       = "MeasurementReferences::AffineTransformationMatrix3d"
	quantityValueMappingFQN   = "MeasurementReferences::QuantityValueMapping"
	definitionalQuantityFQN   = "MeasurementReferences::DefinitionalQuantityValue"
	tensorMRefTypeFQN         = "MeasurementReferences::TensorMeasurementReference"
	orderedCollectionTypeFQN  = "Collections::OrderedCollection"
	frameTransformationRole   = coordinateFrameTypeFQN + "::transformation"
	frameMRefsRole            = tensorMRefTypeFQN + "::mRefs"
	scaleUnitRole             = measurementScaleTypeFQN + "::unit"
	scaleMappingRole          = measurementScaleTypeFQN + "::quantityValueMapping"
	transformationSourceRole  = transformationTypeFQN + "::source"
	transformationTargetRole  = transformationTypeFQN + "::target"
	placementOriginRole       = placementTypeFQN + "::origin"
	placementBasisRole        = placementTypeFQN + "::basisDirections"
	translationVectorRole     = translationTypeFQN + "::translationVector"
	rotationAxisRole          = rotationTypeFQN + "::axisDirection"
	rotationAngleRole         = rotationTypeFQN + "::angle"
	rotationIntrinsicRole     = rotationTypeFQN + "::isIntrinsic"
	affineRotationRole        = affineMatrixTypeFQN + "::rotationMatrix"
	affineTranslationRole     = affineMatrixTypeFQN + "::translationVector"
	mappingMappedRole         = quantityValueMappingFQN + "::mappedQuantityValue"
	mappingReferenceRole      = quantityValueMappingFQN + "::referenceQuantityValue"
	definitionalNumRole       = definitionalQuantityFQN + "::num"
	orderedCollectionElements = orderedCollectionTypeFQN + "::elements"
)

// frameFeatureRoles are the library features a frame, scale or transformation
// object is read by, each keyed by the type declaring it and its name.
var frameFeatureRoles = []string{
	frameTransformationRole, frameMRefsRole, scaleUnitRole, scaleMappingRole,
	transformationSourceRole, transformationTargetRole, placementOriginRole, placementBasisRole,
	translationVectorRole, rotationAxisRole, rotationAngleRole, rotationIntrinsicRole,
	affineRotationRole, affineTranslationRole, mappingMappedRole, mappingReferenceRole,
	definitionalNumRole, orderedCollectionElements,
}

// frameFeatureSymbols memoizes the declaring symbol of each role's feature.
func (ctx *Context) frameFeatureSymbols() map[*symbols.Symbol]string {
	if ctx.frameFeatures != nil {
		return ctx.frameFeatures
	}
	roles := make(map[*symbols.Symbol]string)
	for _, role := range frameFeatureRoles {
		sep := len(role) - len(roleFeatureName(role)) - 2
		typ := ctx.librarySymbol(role[:sep])
		if typ == nil {
			continue
		}
		if feat, ok := ctx.model.LookupMember(typ, roleFeatureName(role)); ok && feat != nil {
			roles[feat] = role
		}
	}
	ctx.frameFeatures = roles
	return roles
}

// roleFeatureName is the feature name a role key ends in.
func roleFeatureName(role string) string {
	for i := len(role) - 1; i > 0; i-- {
		if role[i] == ':' && role[i-1] == ':' {
			return role[i+1:]
		}
	}
	return role
}

// frameRoleOf is the role a member fills: the library feature it is or
// transitively redefines (`:>> mRefs`, `:>> source`); false for another member.
func (ctx *Context) frameRoleOf(member *symbols.Symbol) (string, bool) {
	roles := ctx.frameFeatureSymbols()
	if role, ok := roles[member]; ok {
		return role, true
	}
	for _, redefined := range ctx.model.AllRedefinedFeatures(member) {
		if role, ok := roles[redefined]; ok {
			return role, true
		}
	}
	return "", false
}

// roleElements is the values an object holds under the feature filling a role,
// by whichever name its type declares it, and whether the feature is stated.
func (ctx *Context) roleElements(inst *Instance, role string) ([]Value, bool, error) {
	for _, feat := range ctx.FeaturesOf(inst.Type) {
		if got, ok := ctx.frameRoleOf(feat.Symbol); !ok || got != role {
			continue
		}
		elements, stated, err := ctx.objectFeatureElements(inst, feat.Name)
		// A member whose body describes its object (`:>> transformation { ... }`) states it.
		if stated = stated || bodyDescribesObject(feat.Symbol, feat.Type); err != nil || stated {
			return elements, stated, err
		}
	}
	return nil, false, nil
}

// bodyDescribesObject reports whether a usage's own body restates or adds features
// of a concrete type: a role written so states the one object it describes.
func bodyDescribesObject(sym, typ *symbols.Symbol) bool {
	return sym != nil && typ != nil && !symbols.IsAbstract(typ) && declaresFeatures(sym)
}

// roleValue is the one value an object holds under a role; false when the
// feature is not stated or holds none, an error when it holds several.
func (ctx *Context) roleValue(what string, inst *Instance, role string) (Value, bool, error) {
	elements, stated, err := ctx.roleElements(inst, role)
	if err != nil {
		return Value{}, true, err
	}
	if !stated || len(elements) == 0 {
		return Value{}, false, nil
	}
	if len(elements) > 1 {
		return Value{}, true, fmt.Errorf("%w: %s: %s holds %d values, want one",
			ErrMultiplicityViolation, what, roleFeatureName(role), len(elements))
	}
	return elements[0], true, nil
}

// isFrameType reports a type whose objects are coordinate frames or measurement
// scales: a VectorMeasurementReference that is no MeasurementUnit.
func (ctx *Context) isFrameType(typ *symbols.Symbol) bool {
	vectorRef := ctx.librarySymbol(vectorMRefTypeFQN)
	unit := ctx.librarySymbol(measurementUnitTypeFQN)
	if vectorRef == nil || typ == nil || !ctx.model.Conforms(typ, vectorRef) {
		return false
	}
	return unit == nil || !ctx.model.Conforms(typ, unit)
}

// isTransformationType reports a type whose objects are coordinate transformations.
func (ctx *Context) isTransformationType(typ *symbols.Symbol) bool {
	base := ctx.librarySymbol(transformationTypeFQN)
	return base != nil && typ != nil && ctx.model.Conforms(typ, base)
}

// conformsToLibrary reports whether typ conforms to the library type named.
func (ctx *Context) conformsToLibrary(typ *symbols.Symbol, fqn string) bool {
	base := ctx.librarySymbol(fqn)
	return base != nil && typ != nil && ctx.model.Conforms(typ, base)
}

// referenceValueOfObject reads an object typed by a MeasurementReferences type
// as the frame, scale or transformation value it is; false for another object.
func (ctx *Context) referenceValueOfObject(inst *Instance) (Value, bool, error) {
	if inst == nil || inst.Type == nil {
		return Value{}, false, nil
	}
	typ := ctx.objectType(inst)
	switch {
	case ctx.isFrameType(typ):
		frame, err := ctx.readFrame(inst)
		if err != nil {
			return Value{}, true, err
		}
		return NewCoordinateFrameValue(frame), true, nil
	case ctx.isTransformationType(typ):
		t, err := ctx.readOwnedTransformation(inst)
		if err != nil {
			return Value{}, true, err
		}
		return NewCoordinateTransformationValue(t), true, nil
	}
	return Value{}, false, nil
}

// readOwnedTransformation reads a transformation object: a frame's own
// (`target = that`) through the frame it targets, another on its own.
func (ctx *Context) readOwnedTransformation(inst *Instance) (*CoordinateTransformation, error) {
	if owner, _ := inst.Owner(); owner != nil && ctx.isFrameType(ctx.objectType(owner)) {
		if _, reading := ctx.framesReading[owner.ID]; !reading {
			frame, err := ctx.readFrame(owner)
			if err != nil {
				return nil, err
			}
			if frame.Transformation != nil && frame.Transformation.Object == inst.ID {
				return frame.Transformation, nil
			}
		}
	}
	return ctx.readTransformation(inst, nil)
}

// beginReading marks an object as being read, as frame where one is given; the
// second result undoes it.
func (ctx *Context) beginReading(inst *Instance, what string, frame *CoordinateFrame) (func(), error) {
	if _, reading := ctx.framesReading[inst.ID]; reading {
		return nil, fmt.Errorf("%w: %s %s is defined in terms of itself", ErrCyclicFeatureValue, what, symbolText(inst.Type))
	}
	if ctx.framesReading == nil {
		ctx.framesReading = make(map[int64]*CoordinateFrame)
	}
	ctx.framesReading[inst.ID] = frame
	return func() { delete(ctx.framesReading, inst.ID) }, nil
}

// declOf is the usage an object occurs as; nil for one made of a definition.
func (ctx *Context) declOf(inst *Instance) *symbols.Symbol {
	if ctx.extractType(inst.Type) != nil {
		return inst.Type
	}
	return nil
}

// readFrame reads a frame or scale object: its axes, dimensions, transformation
// and, for a scale, its unit and mapping.
func (ctx *Context) readFrame(inst *Instance) (*CoordinateFrame, error) {
	typ := ctx.objectType(inst)
	frame := &CoordinateFrame{Decl: ctx.declOf(inst), Type: typ, Object: inst.ID, Text: unitSymbolName(inst.Type)}
	if _, feature := inst.Owner(); frame.Decl == nil && feature != "" {
		frame.Text = feature
	}
	done, err := ctx.beginReading(inst, "coordinate frame", frame)
	if err != nil {
		return nil, err
	}
	defer done()
	what := "coordinate frame " + frame.Text
	if ctx.conformsToLibrary(typ, measurementScaleTypeFQN) {
		what = "measurement scale " + frame.Text
		if err := ctx.readScale(what, inst, frame); err != nil {
			return nil, err
		}
	} else if err := ctx.readFrameAxes(what, inst, frame); err != nil {
		return nil, err
	}
	if ctx.conformsToLibrary(typ, coordinateFrameTypeFQN) {
		val, ok, err := ctx.roleValue(what, inst, frameTransformationRole)
		if err != nil {
			return nil, err
		}
		if ok {
			t, err := ctx.transformationOfValue(what+": transformation", val, frame)
			if err != nil {
				return nil, err
			}
			frame.Transformation = t
		}
	}
	return frame, nil
}

// readFrameAxes reads `mRefs` and `dimensions`: the axes are required, the
// dimensions default to the one the axes fill, and the two must agree.
func (ctx *Context) readFrameAxes(what string, inst *Instance, frame *CoordinateFrame) error {
	refs, stated, err := ctx.roleElements(inst, frameMRefsRole)
	if err != nil {
		return err
	}
	if !stated || len(refs) == 0 {
		return fmt.Errorf("%w: %s states no mRefs; TensorMeasurementReference declares mRefs: ScalarMeasurementReference[1..*], one per axis",
			ErrNoValue, what)
	}
	frame.Axes = make([]Unit, len(refs))
	for i, ref := range refs {
		unit, err := axisUnitOf(fmt.Sprintf("%s: mRefs#(%d)", what, i+1), ref)
		if err != nil {
			return err
		}
		frame.Axes[i] = unit
	}
	dims, stated, err := ctx.objectArrayFeature(inst, arrayDimensionsFeature)
	if err != nil {
		return err
	}
	switch {
	case stated:
		frame.Dimensions, err = indexList(what+" dimensions", dims)
		if err != nil {
			return err
		}
	default:
		if fixed, ok := ctx.model.FixedDimensions(ctx.objectType(inst)); ok {
			frame.Dimensions = fixed
		} else {
			frame.Dimensions = []int64{int64(len(refs))}
		}
	}
	size, ok := frame.FlattenedSize()
	if !ok {
		return frame.flattenedSizeError(what)
	}
	if size != int64(len(refs)) {
		return fmt.Errorf("%w: %s states %d mRefs for dimensions %v, whose flattenedSize is %d",
			ErrMultiplicityViolation, what, len(refs), frame.Dimensions, size)
	}
	return nil
}

// axisUnitOf is the unit one `mRefs` element gives an axis: a measurement unit,
// or a scale, whose points are on the scale.
func axisUnitOf(what string, ref Value) (Unit, error) {
	switch ref.Kind {
	case ValMeasurementRef:
		return ref.MeasurementRef().Unit, nil
	case ValCoordinateFrame:
		if frame := ref.CoordinateFrame(); frame.IsScale() {
			return frame.Axes[0], nil
		}
	}
	return Unit{}, fmt.Errorf("%w: %s is %s, want a ScalarMeasurementReference", ErrTypeMismatch, what, describeValue(ref))
}

// readScale reads a scale's `unit` and `quantityValueMapping`; its one axis is
// the scale itself, and its dimensions are empty as ScalarMeasurementReference fixes.
func (ctx *Context) readScale(what string, inst *Instance, frame *CoordinateFrame) error {
	unitVal, ok, err := ctx.roleValue(what, inst, scaleUnitRole)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s states no unit; MeasurementScale declares unit: MeasurementUnit, the unit its magnitudes are in",
			ErrNoValue, what)
	}
	if unitVal.Kind != ValMeasurementRef {
		return fmt.Errorf("%w: %s: unit is %s, want a MeasurementUnit", ErrTypeMismatch, what, describeValue(unitVal))
	}
	anchor := inst.Type
	frame.Scale = &MeasurementScale{Unit: unitVal.MeasurementRef().Unit}
	frame.Axes = []Unit{scaleAnchoredUnit(anchor)}
	mapping, ok, err := ctx.roleValue(what, inst, scaleMappingRole)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	frame.Scale.Mapping, err = ctx.readMapping(what, frame.Scale.Unit, mapping)
	return err
}

// readMapping reads a QuantityValueMapping: the mapped point is in the scale's
// unit, the reference point in the unit of the reference declaring it.
func (ctx *Context) readMapping(what string, scaleUnit Unit, mapping Value) (*QuantityValueMapping, error) {
	inst, err := ctx.objectOfValue(what+": quantityValueMapping", mapping, quantityValueMappingFQN)
	if err != nil {
		return nil, err
	}
	reference := what + ": quantityValueMapping.referenceQuantityValue"
	mapped, err := ctx.definitionalNum(what+": quantityValueMapping.mappedQuantityValue", inst, mappingMappedRole)
	if err != nil {
		return nil, err
	}
	refInst, err := ctx.roleObject(reference, inst, mappingReferenceRole, definitionalQuantityFQN)
	if err != nil {
		return nil, err
	}
	refNum, err := ctx.definitionalNumOf(reference, refInst)
	if err != nil {
		return nil, err
	}
	refUnit, err := ctx.unitDeclaring(reference, refInst)
	if err != nil {
		return nil, err
	}
	return &QuantityValueMapping{
		Mapped:    Quantity{Num: mapped, Unit: scaleUnit},
		Reference: Quantity{Num: refNum, Unit: refUnit},
	}, nil
}

// unitDeclaring is the unit a DefinitionalQuantityValue is declared in — the
// measurement reference owning its declaration (`K.temperatureOfWaterAtTriplePointInK`).
func (ctx *Context) unitDeclaring(what string, inst *Instance) (Unit, error) {
	decl := ctx.declOf(inst)
	if decl == nil || decl.OwnerScope == nil || decl.OwnerScope.Owner() == nil {
		return Unit{}, fmt.Errorf("%w: %s is declared by no measurement reference, so the unit of its num is unknown",
			ErrUnevaluableLibraryFunction, what)
	}
	owner := decl.OwnerScope.Owner()
	if val, ok, err := ctx.MeasurementUnitValue(owner); ok && err == nil {
		switch val.Kind {
		case ValMeasurementRef:
			return val.MeasurementRef().Unit, nil
		case ValCoordinateFrame:
			if frame := val.CoordinateFrame(); frame.IsScale() {
				return frame.Axes[0], nil
			}
		}
	}
	return Unit{}, fmt.Errorf("%w: %s is declared by %s, which is no measurement unit or scale, so the unit of its num is unknown",
		ErrUnevaluableLibraryFunction, what, symbolText(owner))
}

// definitionalNum is the one number a role's DefinitionalQuantityValue states.
func (ctx *Context) definitionalNum(what string, inst *Instance, role string) (semantics.Value, error) {
	point, err := ctx.roleObject(what, inst, role, definitionalQuantityFQN)
	if err != nil {
		return semantics.Value{}, err
	}
	return ctx.definitionalNumOf(what, point)
}

// definitionalNumOf is the one number a DefinitionalQuantityValue's `num` states.
func (ctx *Context) definitionalNumOf(what string, inst *Instance) (semantics.Value, error) {
	num, ok, err := ctx.roleValue(what, inst, definitionalNumRole)
	if err != nil {
		return semantics.Value{}, err
	}
	if !ok {
		return semantics.Value{}, fmt.Errorf("%w: %s states no num", ErrNoValue, what)
	}
	if num.Kind != ValConst || !num.Const.IsNumeric() {
		return semantics.Value{}, fmt.Errorf("%w: %s: num is %s, want a number", ErrTypeMismatch, what, describeValue(num))
	}
	return num.Const, nil
}

// roleObject is the one object a role holds, typed as required.
func (ctx *Context) roleObject(what string, inst *Instance, role, typeFQN string) (*Instance, error) {
	val, ok, err := ctx.roleValue(what, inst, role)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s is not given", ErrNoValue, what)
	}
	return ctx.objectOfValue(what, val, typeFQN)
}

// objectOfValue is the object a value is, typed by the library type named.
func (ctx *Context) objectOfValue(what string, val Value, typeFQN string) (*Instance, error) {
	id, ok := val.Object()
	if !ok {
		return nil, fmt.Errorf("%w: %s is %s, want a %s", ErrTypeMismatch, what, describeValue(val), roleFeatureName(typeFQN))
	}
	inst := ctx.instances[id]
	if inst == nil || inst.Type == nil || !ctx.conformsToLibrary(ctx.objectType(inst), typeFQN) {
		return nil, fmt.Errorf("%w: %s is %s, want a %s", ErrTypeMismatch, what, describeValue(val), roleFeatureName(typeFQN))
	}
	return inst, nil
}

// transformationOfValue is the transformation a value holds: one read already,
// or the object one is read from, as owner's own where owner is given.
func (ctx *Context) transformationOfValue(what string, val Value, owner *CoordinateFrame) (*CoordinateTransformation, error) {
	if val.Kind == ValCoordinateTransformation {
		return val.CoordinateTransformation(), nil
	}
	inst, err := ctx.objectOfValue(what, val, transformationTypeFQN)
	if err != nil {
		return nil, err
	}
	return ctx.readTransformation(inst, owner)
}

// frameOfValue is the frame a value names: one read already, a scalar
// reference's one-dimensional frame, or the object one is read from.
func (ctx *Context) frameOfValue(what string, val Value) (*CoordinateFrame, error) {
	if id, ok := val.Object(); ok {
		// The frame being read names itself as its transformation's target (`target = that`).
		if frame := ctx.framesReading[id]; frame != nil {
			return frame, nil
		}
		inst := ctx.instances[id]
		if inst == nil || !ctx.isFrameType(ctx.objectType(inst)) {
			return nil, fmt.Errorf("%w: %s is %s, want a coordinate frame or a measurement reference",
				ErrTypeMismatch, what, describeValue(val))
		}
		return ctx.readFrame(inst)
	}
	return frameOfReference(what, val)
}

// readTransformation reads a transformation object by the library shape its
// type has; a subtype of none of them is read for its source and target alone.
// A frame's own transformation (owner) targets that frame, as `target = that` binds.
func (ctx *Context) readTransformation(inst *Instance, owner *CoordinateFrame) (*CoordinateTransformation, error) {
	done, err := ctx.beginReading(inst, "coordinate transformation", nil)
	if err != nil {
		return nil, err
	}
	defer done()
	typ := ctx.objectType(inst)
	t := &CoordinateTransformation{Decl: ctx.declOf(inst), Type: typ, Object: inst.ID}
	what := "coordinate transformation " + symbolText(inst.Type)
	if source, ok, err := ctx.roleValue(what, inst, transformationSourceRole); err != nil {
		return nil, err
	} else if ok {
		if t.Source, err = ctx.frameOfValue(what+": source", source); err != nil {
			return nil, err
		}
	}
	if t.Target, err = ctx.readTarget(what, inst, owner); err != nil {
		return nil, err
	}
	switch {
	case ctx.conformsToLibrary(typ, placementTypeFQN):
		err = ctx.readPlacement(what, inst, t)
	case ctx.conformsToLibrary(typ, rotationSequenceTypeFQN):
		err = ctx.readSequence(what, inst, t)
	case ctx.conformsToLibrary(typ, affineMatrixTypeFQN):
		err = ctx.readAffine(what, inst, t)
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// readTarget reads a transformation's `target`: the owning frame for a frame's
// own transformation, whose `target = that` evaluates to the featuring thing, or
// the frame the model states.
func (ctx *Context) readTarget(what string, inst *Instance, owner *CoordinateFrame) (*CoordinateFrame, error) {
	target, ok, err := ctx.roleValue(what, inst, transformationTargetRole)
	if err != nil {
		return nil, err
	}
	if !ok {
		return owner, nil
	}
	if owner != nil && !ctx.namesAFrame(target) {
		return owner, nil
	}
	frame, err := ctx.frameOfValue(what+": target", target)
	if err != nil {
		return nil, err
	}
	if owner != nil && !frame.equal(owner) {
		return nil, fmt.Errorf("%w: %s: target is %s, but the transformation is %s's own, whose target it is",
			ErrTypeMismatch, what, frame.Name(), owner.Name())
	}
	return frame, nil
}

// namesAFrame reports whether a value is, or is an object of, a frame or scalar reference.
func (ctx *Context) namesAFrame(val Value) bool {
	switch val.Kind {
	case ValCoordinateFrame, ValMeasurementRef:
		return true
	}
	if id, ok := val.Object(); ok {
		if _, reading := ctx.framesReading[id]; reading {
			return true
		}
		inst := ctx.instances[id]
		return inst != nil && ctx.isFrameType(ctx.objectType(inst))
	}
	return false
}

// readPlacement reads `origin` and `basisDirections`.
func (ctx *Context) readPlacement(what string, inst *Instance, t *CoordinateTransformation) error {
	origin, ok, err := ctx.roleValue(what, inst, placementOriginRole)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s states no origin; CoordinateFramePlacement declares origin: VectorQuantityValue[1]", ErrNoValue, what)
	}
	basis, _, err := ctx.roleElements(inst, placementBasisRole)
	if err != nil {
		return err
	}
	t.Placement = &FramePlacement{Origin: origin, BasisDirections: basis}
	return nil
}

// readSequence reads the `elements` of a TranslationRotationSequence in order.
func (ctx *Context) readSequence(what string, inst *Instance, t *CoordinateTransformation) error {
	elements, stated, err := ctx.roleElements(inst, orderedCollectionElements)
	if err != nil {
		return err
	}
	if !stated || len(elements) == 0 {
		return fmt.Errorf("%w: %s states no elements; TranslationRotationSequence declares elements: TranslationOrRotation[1..*]",
			ErrNoValue, what)
	}
	t.Sequence = make([]FrameStep, 0, len(elements))
	for i, element := range elements {
		step, err := ctx.readStep(fmt.Sprintf("%s: elements#(%d)", what, i+1), element)
		if err != nil {
			return err
		}
		t.Sequence = append(t.Sequence, step)
	}
	return nil
}

// readStep reads one Translation or Rotation.
func (ctx *Context) readStep(what string, element Value) (FrameStep, error) {
	id, ok := element.Object()
	inst := ctx.instances[id]
	if !ok || inst == nil || inst.Type == nil {
		return FrameStep{}, fmt.Errorf("%w: %s is %s, want a Translation or a Rotation", ErrTypeMismatch, what, describeValue(element))
	}
	typ := ctx.objectType(inst)
	step := FrameStep{Object: inst.ID}
	switch {
	case ctx.conformsToLibrary(typ, translationTypeFQN):
		vec, ok, err := ctx.roleValue(what, inst, translationVectorRole)
		if err != nil {
			return FrameStep{}, err
		}
		if !ok {
			return FrameStep{}, fmt.Errorf("%w: %s states no translationVector", ErrNoValue, what)
		}
		step.Translation = &vec
	case ctx.conformsToLibrary(typ, rotationTypeFQN):
		axis, ok, err := ctx.roleValue(what, inst, rotationAxisRole)
		if err != nil {
			return FrameStep{}, err
		}
		if !ok {
			return FrameStep{}, fmt.Errorf("%w: %s states no axisDirection", ErrNoValue, what)
		}
		angle, ok, err := ctx.roleValue(what, inst, rotationAngleRole)
		if err != nil {
			return FrameStep{}, err
		}
		if !ok {
			return FrameStep{}, fmt.Errorf("%w: %s states no angle", ErrNoValue, what)
		}
		if angle.Kind != ValQuantity {
			return FrameStep{}, fmt.Errorf("%w: %s: angle is %s, want an angular measure such as 90 ['°']",
				ErrTypeMismatch, what, describeValue(angle))
		}
		intrinsic, ok, err := ctx.roleValue(what, inst, rotationIntrinsicRole)
		if err != nil {
			return FrameStep{}, err
		}
		step.Axis, step.Angle, step.Intrinsic = &axis, angle.Quantity(), true
		if ok {
			if intrinsic.Kind != ValConst || intrinsic.Const.Kind != semantics.ValBool {
				return FrameStep{}, fmt.Errorf("%w: %s: isIntrinsic is %s, want a Boolean", ErrTypeMismatch, what, describeValue(intrinsic))
			}
			step.Intrinsic = intrinsic.Const.Bool
		}
	default:
		return FrameStep{}, fmt.Errorf("%w: %s is a %s, which is neither a Translation nor a Rotation",
			ErrUnevaluableLibraryFunction, what, symbolText(typ))
	}
	return step, nil
}

// readAffine reads the `rotationMatrix` and `translationVector` of an
// AffineTransformationMatrix3d as bare numbers.
func (ctx *Context) readAffine(what string, inst *Instance, t *CoordinateTransformation) error {
	affine := &AffineTransformation3d{}
	rotation, err := ctx.roleObject(what+": rotationMatrix", inst, affineRotationRole, arrayTypeFQN)
	if err != nil {
		return err
	}
	if err := ctx.readReals(what+": rotationMatrix", rotation, affine.Rotation[:]); err != nil {
		return err
	}
	translation, err := ctx.roleObject(what+": translationVector", inst, affineTranslationRole, arrayTypeFQN)
	if err != nil {
		return err
	}
	if err := ctx.readReals(what+": translationVector", translation, affine.Translation[:]); err != nil {
		return err
	}
	t.Affine = affine
	return nil
}

// readReals fills out with the elements of an Array object, which must be as many.
func (ctx *Context) readReals(what string, inst *Instance, out []float64) error {
	elements, stated, err := ctx.objectArrayFeature(inst, arrayElementsFeature)
	if err != nil {
		return err
	}
	if !stated {
		return fmt.Errorf("%w: %s states no elements", ErrNoValue, what)
	}
	if len(elements) != len(out) {
		return fmt.Errorf("%w: %s has %d elements, want %d", ErrMultiplicityViolation, what, len(elements), len(out))
	}
	for i, element := range elements {
		if element.Kind != ValConst || !element.Const.IsNumeric() {
			return fmt.Errorf("%w: %s: element %d is %s, want a Real", ErrTypeMismatch, what, i+1, describeValue(element))
		}
		out[i] = element.Const.AsReal()
	}
	return nil
}
