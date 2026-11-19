package grpc

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// coordinateFrameWireModel poses one frame in another three ways, composes a
// frame by arithmetic, and holds a frame as a spatial item's coordinateFrame.
const coordinateFrameWireModel = `
package Frames {
  private import ScalarValues::*;
  private import ISQ::*;
  private import SI::*;
  private import MeasurementReferences::*;
  private import SpatialItems::*;

  attribute datum : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
  attribute lbcf : CartesianSpatial3dCoordinateFrame {
    :>> mRefs = (mm, mm, mm);
    :>> transformation : TranslationRotationSequence {
      :>> source = datum;
      :>> elements = (new Translation((10.0, 0.0, 0.0) [datum]), new Rotation((0.0, 0.0, 1.0) [datum], 180 ['°']));
    }
  }
  attribute placed : CartesianSpatial3dCoordinateFrame {
    :>> mRefs = (mm, mm, mm);
    :>> transformation : CoordinateFramePlacement {
      :>> source = datum;
      :>> origin = (1.0, 2.0, 3.0) [datum];
    }
  }
  attribute velocityCF : CartesianVelocity3dCoordinateFrame[1] = datum / s;
  attribute n : Integer = 3;

  part def Vehicle { attribute body : CoordinateFrame; }
  part vehicle : Vehicle { :>> body = datum / s; }
}
`

func mustCoordinateFrame(t *testing.T, pv *pb.Value) *pb.CoordinateFrame {
	t.Helper()
	frame := pv.GetCoordinateFrame()
	if frame == nil {
		t.Fatalf("value = %v, want the coordinate_frame arm", pv)
	}
	return frame
}

func axisUnits(frame *pb.CoordinateFrame) string {
	units := make([]string, 0, len(frame.GetAxes()))
	for _, axis := range frame.GetAxes() {
		units = append(units, axis.GetUnit())
	}
	return strings.Join(units, ",")
}

func nearly(got, want []float64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-6 {
			return false
		}
	}
	return true
}

// A frame evaluates to the coordinate_frame arm: its declaration, type,
// dimensions and one measurement reference per axis.
func TestEvaluateCoordinateFrameCrossesOnItsArm(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, coordinateFrameWireModel)

	datum := mustCoordinateFrame(t, mustEvaluate(t, srv, modelHash, "Frames::datum"))
	if datum.GetName() != "datum" || datum.GetFrameId() != "Frames::datum" ||
		datum.GetTypeId() != "ISQSpaceTime::CartesianSpatial3dCoordinateFrame" {
		t.Errorf("datum = %v, want it named by its declaration and type", datum)
	}
	if len(datum.GetDimensions()) != 1 || datum.GetDimensions()[0] != 3 || axisUnits(datum) != "mm,mm,mm" {
		t.Errorf("datum = %v, want dimensions [3] over mm, mm, mm", datum)
	}
	if datum.GetAxes()[0].GetUnitId() != "SI::millimetre" {
		t.Errorf("datum axis = %v, want the declared unit SI::millimetre", datum.GetAxes()[0])
	}
	if datum.GetTransformation() != nil || datum.GetScale() != nil {
		t.Errorf("datum = %v, want no transformation and no scale", datum)
	}

	velocity := mustCoordinateFrame(t, mustEvaluate(t, srv, modelHash, "Frames::velocityCF"))
	if axisUnits(velocity) != "mm/s,mm/s,mm/s" {
		t.Errorf("velocityCF axes = %q, want each composed with the second", axisUnits(velocity))
	}

	// A non-frame value keeps its arm.
	if n := mustEvaluate(t, srv, modelHash, "Frames::n"); n.GetIntValue() != 3 {
		t.Errorf("Frames::n = %v, want int 3", n)
	}
	if refs := mustEvaluate(t, srv, modelHash, "Frames::datum.mRefs"); len(refs.GetSequence().GetElements()) != 3 ||
		refs.GetSequence().GetElements()[0].GetMeasurementRef() == nil {
		t.Errorf("Frames::datum.mRefs = %v, want a sequence of three measurement references", refs)
	}
}

// A posed frame carries its transformation: the source frame, the steps as
// written, and the pose they resolve to. The target is the frame itself.
func TestEvaluatePosedFrameCarriesItsTransformation(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, coordinateFrameWireModel)

	lbcf := mustCoordinateFrame(t, mustEvaluate(t, srv, modelHash, "Frames::lbcf"))
	tr := lbcf.GetTransformation()
	if tr == nil || tr.GetTypeId() != "MeasurementReferences::TranslationRotationSequence" {
		t.Fatalf("lbcf.transformation = %v, want a TranslationRotationSequence", tr)
	}
	if tr.GetSource().GetFrameId() != "Frames::datum" || tr.GetTarget() != nil {
		t.Errorf("lbcf.transformation relates %v to %v, want datum to the frame holding it", tr.GetSource(), tr.GetTarget())
	}
	steps := tr.GetSequence().GetSteps()
	if len(steps) != 2 {
		t.Fatalf("steps = %v, want a translation and a rotation", steps)
	}
	shift := steps[0].GetTranslation().GetVectorQuantity()
	if shift == nil || len(shift.GetComponents()) != 3 || shift.GetComponents()[0].GetRealMagnitude() != 10 {
		t.Errorf("step 1 = %v, want the translation ⟨10, 0, 0⟩", steps[0])
	}
	rotation := steps[1].GetRotation()
	if rotation.GetAxisDirection().GetVectorQuantity() == nil || rotation.GetAngle().GetIntMagnitude() != 180 || !rotation.GetIsIntrinsic() {
		t.Errorf("step 2 = %v, want an intrinsic rotation about a vector by 180 degrees", steps[1])
	}
	pose := tr.GetPose()
	if !nearly(pose.GetOrigin(), []float64{10, 0, 0}) || !nearly(pose.GetBasis(), []float64{-1, 0, 0, 0, -1, 0, 0, 0, 1}) {
		t.Errorf("pose = %v, want origin (10, 0, 0) turned half about z", pose)
	}

	// The transformation read on its own names both frames, and its target does
	// not repeat the transformation it was reached through.
	own := mustEvaluate(t, srv, modelHash, "Frames::lbcf.transformation").GetCoordinateTransformation()
	if own == nil {
		t.Fatalf("Frames::lbcf.transformation did not cross on the coordinate_transformation arm")
	}
	if own.GetSource().GetFrameId() != "Frames::datum" || own.GetTarget().GetFrameId() != "Frames::lbcf" {
		t.Errorf("transformation relates %v to %v, want datum to lbcf", own.GetSource(), own.GetTarget())
	}
	if own.GetTarget().GetTransformation() != nil {
		t.Errorf("target repeats its transformation: %v", own.GetTarget())
	}
	if !nearly(own.GetPose().GetOrigin(), []float64{10, 0, 0}) {
		t.Errorf("pose = %v, want origin (10, 0, 0)", own.GetPose())
	}

	placement := mustCoordinateFrame(t, mustEvaluate(t, srv, modelHash, "Frames::placed")).GetTransformation()
	origin := placement.GetPlacement().GetOrigin().GetVectorQuantity()
	if origin == nil || len(origin.GetComponents()) != 3 || origin.GetComponents()[2].GetRealMagnitude() != 3 {
		t.Errorf("placed.transformation = %v, want a placement at ⟨1, 2, 3⟩", placement)
	}
	if !nearly(placement.GetPose().GetOrigin(), []float64{1, 2, 3}) || !nearly(placement.GetPose().GetBasis(), []float64{1, 0, 0, 0, 1, 0, 0, 0, 1}) {
		t.Errorf("placed pose = %v, want origin (1, 2, 3) in the source's orientation", placement.GetPose())
	}
}

// Instantiate answers a feature bound to a frame as the frame, and one
// bound to a posed frame with the transformation posing it.
func TestInstantiateAnswersCoordinateFrames(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, coordinateFrameWireModel)
	ctx := context.Background()

	vehicle, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: modelHash, SymbolId: "Frames::vehicle"})
	if err != nil || vehicle.Error != "" {
		t.Fatalf("Instantiate: err = %v, error = %q", err, vehicle.GetError())
	}
	body := vehicle.Instance.FeatureValues["body"]
	if body == nil || body.Error != "" {
		t.Fatalf("body = %v, want a value", body)
	}
	if frame := mustCoordinateFrame(t, body.Value); axisUnits(frame) != "mm/s,mm/s,mm/s" {
		t.Errorf("body = %v, want a frame over mm/s on each axis", frame)
	}

	pkg, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: modelHash, SymbolId: "Frames"})
	if err != nil || pkg.Error != "" {
		t.Fatalf("Instantiate: err = %v, error = %q", err, pkg.GetError())
	}
	velocity := pkg.Instance.FeatureValues["velocityCF"]
	if velocity == nil || velocity.Error != "" || velocity.Value.GetCoordinateFrame().GetFrameId() != "Frames::velocityCF" {
		t.Errorf("velocityCF = %v, want the frame its declaration names", velocity)
	}
}

// A frame or transformation built with no model behind it crosses with what it
// holds: an affine matrix as bare numbers, a measurement scale as its unit.
func TestValueToProto_CoordinateShapes(t *testing.T) {
	metre := semantics.Unit{Text: "m", Product: semantics.OpaqueUnitProduct("m", semantics.UnitTerm{Scale: semantics.UnitScale(1)})}
	frame := &runtime.CoordinateFrame{Dimensions: []int64{3}, Axes: []runtime.Unit{metre, metre, metre}, Text: "spatialCF"}
	affine := &runtime.AffineTransformation3d{Rotation: [9]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}, Translation: [3]float64{4, 5, 6}}
	pv := protoconv.ValueToProto(runtime.NewCoordinateTransformationValue(&runtime.CoordinateTransformation{Source: frame, Target: frame, Affine: affine}), nil)
	tr := pv.GetCoordinateTransformation()
	if tr == nil || tr.GetSource().GetName() != "spatialCF" || tr.GetTarget().GetName() != "spatialCF" {
		t.Fatalf("transformation = %v, want both frames named", pv)
	}
	if !nearly(tr.GetAffine().GetRotation(), affine.Rotation[:]) || !nearly(tr.GetAffine().GetTranslation(), affine.Translation[:]) {
		t.Errorf("affine = %v, want the matrix and translation stated", tr.GetAffine())
	}
	if !nearly(tr.GetPose().GetOrigin(), []float64{4, 5, 6}) || !nearly(tr.GetPose().GetBasis(), affine.Rotation[:]) {
		t.Errorf("pose = %v, want the affine's own", tr.GetPose())
	}

	bare := protoconv.ValueToProto(runtime.NewCoordinateTransformationValue(&runtime.CoordinateTransformation{}), nil).GetCoordinateTransformation()
	if bare == nil || bare.GetSource() != nil || bare.GetTarget() != nil || bare.GetShape() != nil || bare.GetPose() != nil {
		t.Errorf("a transformation stating nothing = %v, want its name alone", bare)
	}

	scale := &runtime.CoordinateFrame{Text: "UTC", Scale: &runtime.MeasurementScale{Unit: metre, Mapping: &runtime.QuantityValueMapping{
		Mapped:    runtime.Quantity{Num: semantics.Value{Kind: semantics.ValReal, Real: 0}, Unit: metre},
		Reference: runtime.Quantity{Num: semantics.Value{Kind: semantics.ValReal, Real: 2.5}, Unit: metre},
	}}}
	got := protoconv.ValueToProto(runtime.NewCoordinateFrameValue(scale), nil).GetCoordinateFrame().GetScale()
	if got.GetUnit().GetUnit() != "m" || got.GetMapping().GetReference().GetRealMagnitude() != 2.5 {
		t.Errorf("scale = %v, want the unit m mapped to 2.5 m", got)
	}
}

// A frame is read from the model's objects, never supplied, alone or nested.
func TestProtoToValue_RejectsCoordinateArms(t *testing.T) {
	frame := &pb.Value{Kind: &pb.Value_CoordinateFrame{CoordinateFrame: &pb.CoordinateFrame{Name: "f"}}}
	transformation := &pb.Value{Kind: &pb.Value_CoordinateTransformation{CoordinateTransformation: &pb.CoordinateTransformation{Name: "t"}}}
	for _, tc := range []struct {
		arm  *pb.Value
		want error
	}{{frame, protoconv.ErrCoordinateFrameNotAccepted}, {transformation, protoconv.ErrCoordinateTransformationNotAccepted}} {
		if _, err := protoconv.ProtoToValueIn(tc.arm, nil, nil); !errors.Is(err, tc.want) {
			t.Errorf("err = %v, want %v", err, tc.want)
		}
		if _, err := protoconv.ProtoToValueIn(sequenceValue(intValue(1), tc.arm), nil, nil); !errors.Is(err, tc.want) {
			t.Errorf("in a sequence: err = %v, want %v", err, tc.want)
		}
	}
}

// The arms are advertised as a capability; a service without it answers the
// unsupported null naming the frame, as every service did before the arms.
func TestCapabilities_IncludeCoordinateFrameValues(t *testing.T) {
	found := false
	for _, name := range Capabilities() {
		found = found || name == CapabilityCoordinateFrameValues
	}
	if !found {
		t.Errorf("capabilities = %v, want one named %q", Capabilities(), CapabilityCoordinateFrameValues)
	}

	srv := mustNewServiceWithout(t, CapabilityCoordinateFrameValues)
	modelHash := mustParse(t, srv, coordinateFrameWireModel)
	if got := mustEvaluate(t, srv, modelHash, "Frames::datum").GetNull(); got != "unsupported: coordinate frame datum [mm, mm, mm]" {
		t.Errorf("Frames::datum = %q, want the unsupported null naming the frame", got)
	}
	if got := mustEvaluate(t, srv, modelHash, "Frames::lbcf.transformation").GetNull(); !strings.HasPrefix(got, "unsupported: coordinate transformation ") || !strings.Contains(got, "(datum → lbcf)") {
		t.Errorf("Frames::lbcf.transformation = %q, want the unsupported null naming its frames", got)
	}

	// A frame whose placement is a vector the client cannot take is withheld whole.
	srv = mustNewServiceWithout(t, CapabilityStructuredValues)
	modelHash = mustParse(t, srv, coordinateFrameWireModel)
	if got := mustEvaluate(t, srv, modelHash, "Frames::placed").GetNull(); !strings.HasPrefix(got, "unsupported: coordinate frame placed [mm, mm, mm] holding ") {
		t.Errorf("Frames::placed = %q, want the frame withheld for the vector it holds", got)
	}
	if mustEvaluate(t, srv, modelHash, "Frames::datum").GetCoordinateFrame() == nil {
		t.Errorf("Frames::datum holds no vector, and should cross")
	}
}
