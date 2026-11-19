package engine

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

var (
	errCoordinateFrameNotAccepted          = errors.New("a coordinate frame is not a value a caller can supply")
	errCoordinateTransformationNotAccepted = errors.New("a coordinate transformation is not a value a caller can supply")
)

// JCoordinateFrame is the CoordinateFrame message.
type JCoordinateFrame struct {
	Name           string                     `json:"name,omitempty"`
	FrameId        string                     `json:"frameId,omitempty"`
	TypeId         string                     `json:"typeId,omitempty"`
	InstanceId     I64                        `json:"instanceId,omitempty"`
	Dimensions     []I64                      `json:"dimensions,omitempty"`
	Axes           []*JMeasurementRef         `json:"axes,omitempty"`
	Transformation *JCoordinateTransformation `json:"transformation,omitempty"`
	Scale          *JMeasurementScale         `json:"scale,omitempty"`
}

type JMeasurementScale struct {
	Unit    *JMeasurementRef       `json:"unit,omitempty"`
	Mapping *JQuantityValueMapping `json:"mapping,omitempty"`
}

type JQuantityValueMapping struct {
	Mapped    *JQuantity `json:"mapped,omitempty"`
	Reference *JQuantity `json:"reference,omitempty"`
}

// JCoordinateTransformation is the CoordinateTransformation message; at most
// one of Placement, Sequence and Affine is set.
type JCoordinateTransformation struct {
	Name             string                        `json:"name,omitempty"`
	TransformationId string                        `json:"transformationId,omitempty"`
	TypeId           string                        `json:"typeId,omitempty"`
	InstanceId       I64                           `json:"instanceId,omitempty"`
	Source           *JCoordinateFrame             `json:"source,omitempty"`
	Target           *JCoordinateFrame             `json:"target,omitempty"`
	Placement        *JFramePlacement              `json:"placement,omitempty"`
	Sequence         *JTranslationRotationSequence `json:"sequence,omitempty"`
	Affine           *JAffineTransformation3d      `json:"affine,omitempty"`
	Pose             *JFramePose                   `json:"pose,omitempty"`
}

type JFramePlacement struct {
	Origin          *JValue   `json:"origin,omitempty"`
	BasisDirections []*JValue `json:"basisDirections,omitempty"`
}

type JTranslationRotationSequence struct {
	Steps []*JFrameStep `json:"steps,omitempty"`
}

// JFrameStep is the FrameStep message; exactly one of Translation and Rotation is set.
type JFrameStep struct {
	Translation *JValue         `json:"translation,omitempty"`
	Rotation    *JFrameRotation `json:"rotation,omitempty"`
	InstanceId  I64             `json:"instanceId,omitempty"`
}

type JFrameRotation struct {
	AxisDirection *JValue    `json:"axisDirection,omitempty"`
	Angle         *JQuantity `json:"angle,omitempty"`
	IsIntrinsic   bool       `json:"isIntrinsic,omitempty"`
}

type JFramePose struct {
	Origin []F64 `json:"origin,omitempty"`
	Basis  []F64 `json:"basis,omitempty"`
}

type JAffineTransformation3d struct {
	Rotation    []F64 `json:"rotation,omitempty"`
	Translation []F64 `json:"translation,omitempty"`
}

// frameEncoder marshals one frame or transformation value with everything it
// reaches; unsupported is the reason of the first part with no wire form.
type frameEncoder struct {
	rt          *runtime.Context
	idx         *symbols.Index
	ids         elementIdentities
	unsupported string
}

// coordinateValueToProto sends a frame or transformation through its arm, or
// the unsupported null when a quantity or vector it holds has no wire form.
func coordinateValueToProto(rt *runtime.Context, val runtime.Value, idx *symbols.Index, ids elementIdentities) *JValue {
	enc := &frameEncoder{rt: rt, idx: idx, ids: ids}
	out := &JValue{}
	if val.Kind == runtime.ValCoordinateFrame {
		out.CoordinateFrame = enc.frame(val.CoordinateFrame(), nil)
	} else {
		out.CoordinateTransformation = enc.transformation(val.CoordinateTransformation(), nil)
	}
	if enc.unsupported != "" {
		return &JValue{Null: text(unsupportedShown(val) + " holding " + enc.unsupported)}
	}
	return out
}

// frame marshals f; within is the transformation whose target f is being
// marshalled as, which f does not repeat as its own.
func (e *frameEncoder) frame(f *runtime.CoordinateFrame, within *runtime.CoordinateTransformation) *JCoordinateFrame {
	if f == nil {
		return nil
	}
	out := &JCoordinateFrame{
		Name:       f.Name(),
		FrameId:    elementIdentity(f.Decl, e.idx, e.ids),
		TypeId:     e.typeID(f.Type),
		InstanceId: I64(f.Object),
	}
	for _, dimension := range f.Dimensions {
		out.Dimensions = append(out.Dimensions, I64(dimension))
	}
	for _, axis := range f.Axes {
		out.Axes = append(out.Axes, measurementRefToProto(&runtime.MeasurementRef{Unit: axis}))
	}
	if f.Scale != nil {
		out.Scale = &JMeasurementScale{Unit: measurementRefToProto(&runtime.MeasurementRef{Unit: f.Scale.Unit})}
		if m := f.Scale.Mapping; m != nil {
			out.Scale.Mapping = &JQuantityValueMapping{Mapped: e.quantity(&m.Mapped), Reference: e.quantity(&m.Reference)}
		}
	}
	if t := f.Transformation; t != nil && !sameTransformation(t, within) {
		out.Transformation = e.transformation(t, f)
	}
	return out
}

// transformation marshals t; owner is the frame whose own transformation t is
// being marshalled as, which t does not repeat as its target.
func (e *frameEncoder) transformation(t *runtime.CoordinateTransformation, owner *runtime.CoordinateFrame) *JCoordinateTransformation {
	if t == nil {
		return nil
	}
	out := &JCoordinateTransformation{
		Name:             t.Name(),
		TransformationId: elementIdentity(t.Decl, e.idx, e.ids),
		TypeId:           e.typeID(t.Type),
		InstanceId:       I64(t.Object),
		Source:           e.frame(t.Source, nil),
	}
	if owner == nil {
		out.Target = e.frame(t.Target, t)
	}
	switch {
	case t.Placement != nil:
		out.Placement = &JFramePlacement{Origin: e.value(t.Placement.Origin)}
		for _, direction := range t.Placement.BasisDirections {
			out.Placement.BasisDirections = append(out.Placement.BasisDirections, e.value(direction))
		}
	case t.Sequence != nil:
		out.Sequence = &JTranslationRotationSequence{}
		for _, step := range t.Sequence {
			out.Sequence.Steps = append(out.Sequence.Steps, e.step(step))
		}
	case t.Affine != nil:
		out.Affine = &JAffineTransformation3d{}
		for _, x := range t.Affine.Rotation {
			out.Affine.Rotation = append(out.Affine.Rotation, F64(x))
		}
		for _, x := range t.Affine.Translation {
			out.Affine.Translation = append(out.Affine.Translation, F64(x))
		}
	}
	if pose, ok := e.rt.TransformationPose(t); ok {
		out.Pose = &JFramePose{}
		for _, x := range pose.Origin {
			out.Pose.Origin = append(out.Pose.Origin, F64(x))
		}
		for _, x := range pose.Basis {
			out.Pose.Basis = append(out.Pose.Basis, F64(x))
		}
	}
	return out
}

// sameTransformation holds for the one transformation read twice: a frame's
// own, and the same one reached again through the frame it targets.
func sameTransformation(t, other *runtime.CoordinateTransformation) bool {
	return other != nil && (t == other || t.Object != 0 && t.Object == other.Object)
}

func (e *frameEncoder) step(step runtime.FrameStep) *JFrameStep {
	out := &JFrameStep{InstanceId: I64(step.Object)}
	if step.Translation != nil {
		out.Translation = e.value(*step.Translation)
		return out
	}
	out.Rotation = &JFrameRotation{IsIntrinsic: step.Intrinsic}
	if step.Axis != nil {
		out.Rotation.AxisDirection = e.value(*step.Axis)
	}
	if step.Angle != nil {
		out.Rotation.Angle = e.quantity(step.Angle)
	}
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

func (e *frameEncoder) value(val runtime.Value) *JValue {
	out := valueToProto(e.rt, val, e.idx, e.ids)
	if reason, ok := unsupportedReason(out); ok && e.unsupported == "" {
		e.unsupported = reason
	}
	return out
}

func (e *frameEncoder) quantity(q *runtime.Quantity) *JQuantity {
	out := quantityToProto(q)
	if out == nil && e.unsupported == "" {
		e.unsupported = "quantity with a non-numeric magnitude"
	}
	return out
}
