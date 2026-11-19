package protoconv

import (
	"errors"
	"slices"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

var (
	// ErrCoordinateFrameNotAccepted reports the coordinate_frame arm arriving as
	// an input: a frame is read from the model's own objects, not supplied.
	ErrCoordinateFrameNotAccepted = errors.New("a coordinate frame is not a value a caller can supply")

	// ErrCoordinateTransformationNotAccepted reports the coordinate_transformation
	// arm arriving as an input.
	ErrCoordinateTransformationNotAccepted = errors.New("a coordinate transformation is not a value a caller can supply")
)

// frameEncoder marshals one frame or transformation value with everything it
// reaches. unsupported is the reason of the first part with no wire form,
// which withholds the whole value: a frame short of an origin is another frame.
type frameEncoder struct {
	rt          *runtime.Context
	idx         *symbols.Index
	ids         ElementIdentities
	unsupported string
}

// coordinateValueToProto sends a frame through the coordinate_frame arm and a
// transformation through coordinate_transformation, or the unsupported null
// when a quantity or vector either holds has no wire form.
func coordinateValueToProto(rt *runtime.Context, val runtime.Value, idx *symbols.Index, ids ElementIdentities) *pb.Value {
	enc := &frameEncoder{rt: rt, idx: idx, ids: ids}
	var out *pb.Value
	if val.Kind == runtime.ValCoordinateFrame {
		out = &pb.Value{Kind: &pb.Value_CoordinateFrame{CoordinateFrame: enc.frame(val.CoordinateFrame(), nil)}}
	} else {
		out = &pb.Value{Kind: &pb.Value_CoordinateTransformation{CoordinateTransformation: enc.transformation(val.CoordinateTransformation(), nil)}}
	}
	if enc.unsupported != "" {
		return &pb.Value{Kind: &pb.Value_Null{Null: UnsupportedShown(val).Null + " holding " + enc.unsupported}}
	}
	return out
}

// frame marshals f; within is the transformation whose target f is being
// marshalled as, which f does not repeat as its own.
func (e *frameEncoder) frame(f *runtime.CoordinateFrame, within *runtime.CoordinateTransformation) *pb.CoordinateFrame {
	if f == nil {
		return nil
	}
	out := &pb.CoordinateFrame{
		Name:       f.Name(),
		FrameId:    ElementIdentity(f.Decl, e.idx, e.ids),
		TypeId:     e.typeID(f.Type),
		InstanceId: f.Object,
		Dimensions: slices.Clone(f.Dimensions),
	}
	for _, axis := range f.Axes {
		out.Axes = append(out.Axes, MeasurementRefToProto(&runtime.MeasurementRef{Unit: axis}))
	}
	if f.Scale != nil {
		out.Scale = &pb.MeasurementScale{Unit: MeasurementRefToProto(&runtime.MeasurementRef{Unit: f.Scale.Unit})}
		if m := f.Scale.Mapping; m != nil {
			out.Scale.Mapping = &pb.QuantityValueMapping{Mapped: e.quantity(&m.Mapped), Reference: e.quantity(&m.Reference)}
		}
	}
	if t := f.Transformation; t != nil && !sameTransformation(t, within) {
		out.Transformation = e.transformation(t, f)
	}
	return out
}

// transformation marshals t; owner is the frame whose own transformation t is
// being marshalled as, which t does not repeat as its target.
func (e *frameEncoder) transformation(t *runtime.CoordinateTransformation, owner *runtime.CoordinateFrame) *pb.CoordinateTransformation {
	if t == nil {
		return nil
	}
	out := &pb.CoordinateTransformation{
		Name:             t.Name(),
		TransformationId: ElementIdentity(t.Decl, e.idx, e.ids),
		TypeId:           e.typeID(t.Type),
		InstanceId:       t.Object,
		Source:           e.frame(t.Source, nil),
	}
	if owner == nil {
		out.Target = e.frame(t.Target, t)
	}
	switch {
	case t.Placement != nil:
		placement := &pb.FramePlacement{Origin: e.value(t.Placement.Origin)}
		for _, direction := range t.Placement.BasisDirections {
			placement.BasisDirections = append(placement.BasisDirections, e.value(direction))
		}
		out.Shape = &pb.CoordinateTransformation_Placement{Placement: placement}
	case t.Sequence != nil:
		sequence := &pb.TranslationRotationSequence{}
		for _, step := range t.Sequence {
			sequence.Steps = append(sequence.Steps, e.step(step))
		}
		out.Shape = &pb.CoordinateTransformation_Sequence{Sequence: sequence}
	case t.Affine != nil:
		out.Shape = &pb.CoordinateTransformation_Affine{Affine: &pb.AffineTransformation3D{
			Rotation:    slices.Clone(t.Affine.Rotation[:]),
			Translation: slices.Clone(t.Affine.Translation[:]),
		}}
	}
	if pose, ok := e.rt.TransformationPose(t); ok {
		out.Pose = &pb.FramePose{Origin: pose.Origin, Basis: pose.Basis}
	}
	return out
}

// sameTransformation holds for the one transformation read twice: a frame's
// own, and the same one reached again through the frame it targets.
func sameTransformation(t, other *runtime.CoordinateTransformation) bool {
	return other != nil && (t == other || t.Object != 0 && t.Object == other.Object)
}

func (e *frameEncoder) step(step runtime.FrameStep) *pb.FrameStep {
	out := &pb.FrameStep{InstanceId: step.Object}
	if step.Translation != nil {
		out.Step = &pb.FrameStep_Translation{Translation: e.value(*step.Translation)}
		return out
	}
	rotation := &pb.FrameRotation{IsIntrinsic: step.Intrinsic}
	if step.Axis != nil {
		rotation.AxisDirection = e.value(*step.Axis)
	}
	if step.Angle != nil {
		rotation.Angle = e.quantity(step.Angle)
	}
	out.Step = &pb.FrameStep_Rotation{Rotation: rotation}
	return out
}

func (e *frameEncoder) typeID(typ *symbols.Symbol) string {
	if typ == nil || !typ.Named() {
		return ""
	}
	if e.idx != nil {
		return e.idx.GetFQN(typ)
	}
	return typ.Name
}

func (e *frameEncoder) value(val runtime.Value) *pb.Value {
	out := ValueToProtoWith(e.rt, val, e.idx, e.ids)
	if reason, ok := UnsupportedReason(out); ok && e.unsupported == "" {
		e.unsupported = reason
	}
	return out
}

func (e *frameEncoder) quantity(q *runtime.Quantity) *pb.Quantity {
	out := QuantityToProto(q)
	if out == nil && e.unsupported == "" {
		e.unsupported = "quantity with a non-numeric magnitude"
	}
	return out
}

// coordinateParts lists the Values and the Quantities a frame or
// transformation value holds at any depth of the frames it relates; both are
// empty for any other value.
func coordinateParts(pv *pb.Value) (values []*pb.Value, quantities []*pb.Quantity) {
	var frame func(*pb.CoordinateFrame)
	var transformation func(*pb.CoordinateTransformation)
	frame = func(f *pb.CoordinateFrame) {
		if f == nil {
			return
		}
		if m := f.GetScale().GetMapping(); m != nil {
			quantities = append(quantities, presentQuantities(m.GetMapped(), m.GetReference())...)
		}
		transformation(f.GetTransformation())
	}
	transformation = func(t *pb.CoordinateTransformation) {
		if t == nil {
			return
		}
		frame(t.GetSource())
		frame(t.GetTarget())
		switch shape := t.GetShape().(type) {
		case *pb.CoordinateTransformation_Placement:
			values = append(values, presentValues(shape.Placement.GetOrigin())...)
			values = append(values, presentValues(shape.Placement.GetBasisDirections()...)...)
		case *pb.CoordinateTransformation_Sequence:
			for _, step := range shape.Sequence.GetSteps() {
				switch s := step.GetStep().(type) {
				case *pb.FrameStep_Translation:
					values = append(values, presentValues(s.Translation)...)
				case *pb.FrameStep_Rotation:
					values = append(values, presentValues(s.Rotation.GetAxisDirection())...)
					quantities = append(quantities, presentQuantities(s.Rotation.GetAngle())...)
				}
			}
		}
	}
	switch k := pv.GetKind().(type) {
	case *pb.Value_CoordinateFrame:
		frame(k.CoordinateFrame)
	case *pb.Value_CoordinateTransformation:
		transformation(k.CoordinateTransformation)
	}
	return values, quantities
}

func presentValues(values ...*pb.Value) []*pb.Value {
	return slices.DeleteFunc(slices.Clone(values), func(v *pb.Value) bool { return v == nil })
}

func presentQuantities(quantities ...*pb.Quantity) []*pb.Quantity {
	return slices.DeleteFunc(slices.Clone(quantities), func(q *pb.Quantity) bool { return q == nil })
}

// ValueCarriesCoordinateFrame reports whether a value, or any value nested in
// it, is a CoordinateFrame or a CoordinateTransformation: the kinds
// coordinate_frame_values governs.
func ValueCarriesCoordinateFrame(pv *pb.Value) bool {
	return valueCarries(pv, IsCoordinateValue)
}

// IsCoordinateValue reports whether a value is itself a CoordinateFrame or a
// CoordinateTransformation.
func IsCoordinateValue(pv *pb.Value) bool {
	switch pv.GetKind().(type) {
	case *pb.Value_CoordinateFrame, *pb.Value_CoordinateTransformation:
		return true
	}
	return false
}

// CoordinateValueName is the name a frame or transformation value crosses
// under; "" for any other value.
func CoordinateValueName(pv *pb.Value) string {
	switch k := pv.GetKind().(type) {
	case *pb.Value_CoordinateFrame:
		return k.CoordinateFrame.GetName()
	case *pb.Value_CoordinateTransformation:
		return k.CoordinateTransformation.GetName()
	}
	return ""
}
