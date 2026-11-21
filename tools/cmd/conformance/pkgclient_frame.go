package main

import (
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// frameToProto writes a frame the public API answered back as the wire message
// the suite compares.
func frameToProto(f *opensysml.CoordinateFrame) *pb.CoordinateFrame {
	if f == nil {
		return nil
	}
	out := &pb.CoordinateFrame{
		Name: f.Name, FrameId: f.FrameID, TypeId: f.TypeID, InstanceId: f.InstanceID,
		Dimensions:     f.Dimensions,
		Transformation: transformationToProto(f.Transformation),
	}
	for _, axis := range f.Axes {
		out.Axes = append(out.Axes, measurementRefToProto(axis))
	}
	if f.Scale != nil {
		out.Scale = &pb.MeasurementScale{Unit: measurementRefToProto(f.Scale.Unit)}
		if m := f.Scale.Mapping; m != nil {
			out.Scale.Mapping = &pb.QuantityValueMapping{Mapped: quantityToProto(m.Mapped), Reference: quantityToProto(m.Reference)}
		}
	}
	return out
}

func measurementRefToProto(ref opensysml.MeasurementRef) *pb.MeasurementRef {
	return &pb.MeasurementRef{Unit: ref.Unit, UnitTerm: unitTermToProto(ref.Term), UnitId: ref.UnitID}
}

// transformationToProto writes a transformation the public API answered back
// as the wire message the suite compares.
func transformationToProto(t *opensysml.CoordinateTransformation) *pb.CoordinateTransformation {
	if t == nil {
		return nil
	}
	out := &pb.CoordinateTransformation{
		Name: t.Name, TransformationId: t.TransformationID, TypeId: t.TypeID, InstanceId: t.InstanceID,
		Source: frameToProto(t.Source), Target: frameToProto(t.Target),
	}
	switch {
	case t.Placement != nil:
		placement := &pb.FramePlacement{Origin: valueToProto(t.Placement.Origin)}
		for _, direction := range t.Placement.BasisDirections {
			placement.BasisDirections = append(placement.BasisDirections, valueToProto(direction))
		}
		out.Shape = &pb.CoordinateTransformation_Placement{Placement: placement}
	case t.Steps != nil:
		sequence := &pb.TranslationRotationSequence{}
		for _, step := range t.Steps {
			written := &pb.FrameStep{InstanceId: step.InstanceID}
			if step.Rotation != nil {
				written.Step = &pb.FrameStep_Rotation{Rotation: &pb.FrameRotation{
					AxisDirection: valueToProto(step.Rotation.AxisDirection),
					Angle:         quantityToProto(step.Rotation.Angle),
					IsIntrinsic:   step.Rotation.Intrinsic,
				}}
			} else {
				written.Step = &pb.FrameStep_Translation{Translation: valueToProto(step.Translation)}
			}
			sequence.Steps = append(sequence.Steps, written)
		}
		out.Shape = &pb.CoordinateTransformation_Sequence{Sequence: sequence}
	case t.Affine != nil:
		out.Shape = &pb.CoordinateTransformation_Affine{Affine: &pb.AffineTransformation3D{
			Rotation: t.Affine.Rotation, Translation: t.Affine.Translation,
		}}
	}
	if t.Pose != nil {
		out.Pose = &pb.FramePose{Origin: t.Pose.Origin, Basis: t.Pose.Basis}
	}
	return out
}
