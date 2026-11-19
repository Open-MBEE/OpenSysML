package opensysml

import (
	"reflect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// CoordinateFrame is a coordinate frame or measurement scale held as a value:
// what a feature typed by a MeasurementReferences::CoordinateFrame evaluates
// to. It is reported, never accepted.
type CoordinateFrame struct {
	// Name is the frame as a reader names it: its declared name, else the
	// expression that composed it ("datum / s").
	Name string
	// FrameID identifies the usage declaring the frame; empty for one composed.
	FrameID string
	// TypeID is the qualified name of the frame's type; empty for none.
	TypeID string
	// InstanceID is the object the frame was read from; 0 for none.
	InstanceID int64
	// Dimensions are the frame's `dimensions`, and Axes its `mRefs`, one per axis.
	Dimensions []int64
	Axes       []MeasurementRef
	// Transformation places the frame in another; nil for a frame stating none.
	// Its Target is nil: the frame holding it is its target.
	Transformation *CoordinateTransformation
	// Scale is set for a measurement scale, which has no axes.
	Scale *MeasurementScale
}

// MeasurementScale is the unit a scale is measured in and, when the model
// states one, the point defining it against another reference.
type MeasurementScale struct {
	Unit    MeasurementRef
	Mapping *QuantityValueMapping
}

// QuantityValueMapping is one point on a scale and the same point on the
// reference the scale is defined against.
type QuantityValueMapping struct {
	Mapped, Reference Quantity
}

// CoordinateTransformation is a coordinate transformation held as a value: the
// frames it relates, the shape the model states it in, and the pose that
// shape resolves to. At most one of Placement, Steps and Affine is set. It is
// reported, never accepted.
type CoordinateTransformation struct {
	// Name is the transformation as a reader names it: its declared name, else its type.
	Name string
	// TransformationID identifies the usage declaring it; empty for none.
	TransformationID string
	// TypeID is the qualified name of the transformation's type; empty for none.
	TypeID string
	// InstanceID is the object the transformation was read from; 0 for none.
	InstanceID int64
	// Source and Target are the frames related; nil where the model states none.
	Source, Target *CoordinateFrame
	// Placement is set for a CoordinateFramePlacement.
	Placement *FramePlacement
	// Steps are the elements of a TranslationRotationSequence, in the order applied.
	Steps []FrameStep
	// Affine is set for an AffineTransformationMatrix3d.
	Affine *AffineTransformation3D
	// Pose is where the target sits in the source; nil when no source is
	// stated or the shape resolves to none.
	Pose *FramePose
}

// FramePlacement is the target frame's origin in the source frame and, when
// stated, its basis directions there, one per axis.
type FramePlacement struct {
	Origin          Value
	BasisDirections []Value
}

// FrameStep is one step of a TranslationRotationSequence: a translation when
// Translation is set, otherwise a rotation.
type FrameStep struct {
	Translation Value
	Rotation    *FrameRotation
	// InstanceID is the Translation or Rotation object the step was read from.
	InstanceID int64
}

// FrameRotation turns by Angle about AxisDirection: about the axes as already
// rotated when Intrinsic, about the source's fixed axes otherwise.
type FrameRotation struct {
	AxisDirection Value
	Angle         Quantity
	Intrinsic     bool
}

// AffineTransformation3D is a 3×3 rotation matrix, row-major, and a translation.
type AffineTransformation3D struct {
	Rotation    []float64
	Translation []float64
}

// FramePose is where a target frame sits in its source frame, in the source's
// axis units: target coordinates v lie at Origin + Basis·v, Basis row-major.
type FramePose struct {
	Origin []float64
	Basis  []float64
}

// String renders the frame as the service prints it: `datum [mm, mm, mm]`.
func (f *CoordinateFrame) String() string {
	if f == nil {
		return "null"
	}
	out := f.Name + " ["
	if f.Scale != nil {
		return out + f.Scale.Unit.String() + "]"
	}
	for i, axis := range f.Axes {
		if i > 0 {
			out += ", "
		}
		out += axis.String()
	}
	return out + "]"
}

// String renders the transformation over the frames it relates: `trs (datum → lbcf)`.
func (t *CoordinateTransformation) String() string {
	if t == nil {
		return "null"
	}
	source, target := "?", "?"
	if t.Source != nil {
		source = t.Source.Name
	}
	if t.Target != nil {
		target = t.Target.Name
	}
	return t.Name + " (" + source + " → " + target + ")"
}

func (*CoordinateFrame) isValue()          { /* marker: closed Value set */ }
func (*CoordinateTransformation) isValue() { /* marker: closed Value set */ }

// coordinatesEqual compares two frames or transformations by everything sent of them.
func coordinatesEqual(a, b Value) bool {
	return reflect.DeepEqual(a, b)
}

// frameFromProto reads a frame; ok is false when a part of it cannot be read.
func frameFromProto(pf *pb.CoordinateFrame) (*CoordinateFrame, bool) {
	if pf == nil {
		return nil, true
	}
	out := &CoordinateFrame{
		Name: pf.GetName(), FrameID: pf.GetFrameId(), TypeID: pf.GetTypeId(),
		InstanceID: pf.GetInstanceId(), Dimensions: pf.GetDimensions(),
	}
	for _, axis := range pf.GetAxes() {
		ref, ok := valueFromProto(&pb.Value{Kind: &pb.Value_MeasurementRef{MeasurementRef: axis}}).(MeasurementRef)
		if !ok {
			return nil, false
		}
		out.Axes = append(out.Axes, ref)
	}
	if scale := pf.GetScale(); scale != nil {
		unit, ok := valueFromProto(&pb.Value{Kind: &pb.Value_MeasurementRef{MeasurementRef: scale.GetUnit()}}).(MeasurementRef)
		if !ok {
			return nil, false
		}
		out.Scale = &MeasurementScale{Unit: unit}
		if mapping := scale.GetMapping(); mapping != nil {
			mapped, okMapped := quantityFromProto(mapping.GetMapped())
			reference, okReference := quantityFromProto(mapping.GetReference())
			if !okMapped || !okReference {
				return nil, false
			}
			out.Scale.Mapping = &QuantityValueMapping{Mapped: mapped, Reference: reference}
		}
	}
	var ok bool
	if out.Transformation, ok = transformationFromProto(pf.GetTransformation()); !ok {
		return nil, false
	}
	return out, true
}

// transformationFromProto reads a transformation; ok is false when a part of
// it cannot be read.
func transformationFromProto(pt *pb.CoordinateTransformation) (*CoordinateTransformation, bool) {
	if pt == nil {
		return nil, true
	}
	out := &CoordinateTransformation{
		Name: pt.GetName(), TransformationID: pt.GetTransformationId(), TypeID: pt.GetTypeId(),
		InstanceID: pt.GetInstanceId(),
	}
	var ok bool
	if out.Source, ok = frameFromProto(pt.GetSource()); !ok {
		return nil, false
	}
	if out.Target, ok = frameFromProto(pt.GetTarget()); !ok {
		return nil, false
	}
	switch shape := pt.GetShape().(type) {
	case *pb.CoordinateTransformation_Placement:
		out.Placement = &FramePlacement{Origin: valueFromProto(shape.Placement.GetOrigin())}
		for _, direction := range shape.Placement.GetBasisDirections() {
			out.Placement.BasisDirections = append(out.Placement.BasisDirections, valueFromProto(direction))
		}
	case *pb.CoordinateTransformation_Sequence:
		out.Steps = make([]FrameStep, 0, len(shape.Sequence.GetSteps()))
		for _, step := range shape.Sequence.GetSteps() {
			read := FrameStep{InstanceID: step.GetInstanceId()}
			switch s := step.GetStep().(type) {
			case *pb.FrameStep_Translation:
				read.Translation = valueFromProto(s.Translation)
			case *pb.FrameStep_Rotation:
				angle, ok := quantityFromProto(s.Rotation.GetAngle())
				if !ok {
					return nil, false
				}
				read.Rotation = &FrameRotation{
					AxisDirection: valueFromProto(s.Rotation.GetAxisDirection()),
					Angle:         angle,
					Intrinsic:     s.Rotation.GetIsIntrinsic(),
				}
			default:
				return nil, false
			}
			out.Steps = append(out.Steps, read)
		}
	case *pb.CoordinateTransformation_Affine:
		out.Affine = &AffineTransformation3D{Rotation: shape.Affine.GetRotation(), Translation: shape.Affine.GetTranslation()}
	}
	if pose := pt.GetPose(); pose != nil {
		out.Pose = &FramePose{Origin: pose.GetOrigin(), Basis: pose.GetBasis()}
	}
	return out, true
}
