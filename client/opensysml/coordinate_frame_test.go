package opensysml_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const coordinateFrameSource = `package Frames {
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::*;

	attribute datum : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
	attribute lbcf : CartesianSpatial3dCoordinateFrame {
		:>> mRefs = (mm, mm, mm);
		:>> transformation : TranslationRotationSequence {
			:>> source = datum;
			:>> elements = (new Translation((10.0, 0.0, 0.0) [datum]), new Rotation((0.0, 0.0, 1.0) [datum], 180 ['°']));
		}
	}
	attribute n = 3;
}`

// A frame arrives as a *CoordinateFrame over every transport, a posed one with
// the transformation posing it, and neither is a value a caller can send back.
func TestCoordinateFramesCrossEveryTransport(t *testing.T) {
	address := startService(t)
	for name, client := range map[string]opensysml.Client{
		"in-process":    newClient(t),
		"connect-proto": dialClient(t, address),
		"connect-json":  dialClient(t, address, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			info, err := client.ServerInfo(ctx)
			if err != nil {
				t.Fatalf("ServerInfo: %v", err)
			}
			if !info.Has(opensysml.CapabilityCoordinateFrameValues) {
				t.Errorf("capabilities %v do not name %s", info.Capabilities, opensysml.CapabilityCoordinateFrameValues)
			}
			model := parse(t, client, coordinateFrameSource)

			got, err := client.Evaluate(ctx, model, "Frames::datum")
			if err != nil {
				t.Fatalf("Evaluate(datum): %v", err)
			}
			datum, ok := got.(*opensysml.CoordinateFrame)
			if !ok || datum.FrameID != "Frames::datum" || len(datum.Axes) != 3 || datum.Axes[0].UnitID != "SI::millimetre" {
				t.Fatalf("datum = %#v, want the frame Frames::datum over three millimetre axes", got)
			}
			if fmt.Sprint(got) != "datum [mm, mm, mm]" {
				t.Errorf("datum renders as %q, want datum [mm, mm, mm]", fmt.Sprint(got))
			}

			got, err = client.Evaluate(ctx, model, "Frames::lbcf")
			if err != nil {
				t.Fatalf("Evaluate(lbcf): %v", err)
			}
			lbcf, ok := got.(*opensysml.CoordinateFrame)
			if !ok || lbcf.Transformation == nil {
				t.Fatalf("lbcf = %#v, want a frame carrying its transformation", got)
			}
			tr := lbcf.Transformation
			if tr.Source == nil || tr.Source.FrameID != "Frames::datum" || tr.Target != nil || len(tr.Steps) != 2 {
				t.Errorf("lbcf.transformation = %#v, want two steps from datum", tr)
			}
			if _, ok := tr.Steps[0].Translation.(opensysml.VectorQuantity); !ok || tr.Steps[1].Rotation == nil {
				t.Errorf("steps = %#v, want a translation vector then a rotation", tr.Steps)
			}
			if tr.Pose == nil || len(tr.Pose.Basis) != 9 || tr.Pose.Origin[0] != 10 || math.Abs(tr.Pose.Basis[0]+1) > 1e-6 {
				t.Errorf("pose = %#v, want origin (10, 0, 0) turned half about z", tr.Pose)
			}
			if !opensysml.Equal(got, lbcf) || opensysml.Equal(got, datum) {
				t.Errorf("a frame equals itself and no other frame")
			}

			got, err = client.Evaluate(ctx, model, "Frames::lbcf.transformation")
			if err != nil {
				t.Fatalf("Evaluate(lbcf.transformation): %v", err)
			}
			own, ok := got.(*opensysml.CoordinateTransformation)
			if !ok || own.Target == nil || own.Target.FrameID != "Frames::lbcf" || own.Target.Transformation != nil {
				t.Fatalf("lbcf.transformation = %#v, want it to name lbcf as its target once", got)
			}
			if fmt.Sprint(got) != own.Name+" (datum → lbcf)" {
				t.Errorf("transformation renders as %q", fmt.Sprint(got))
			}

			if n, err := client.Evaluate(ctx, model, "Frames::n"); err != nil || n != opensysml.Int(3) {
				t.Errorf("Frames::n = %#v, %v, want Int 3", n, err)
			}

			var status *opensysml.StatusError
			for _, value := range []opensysml.Value{datum, own} {
				_, err := client.EvaluateCalc(ctx, model, "Frames::n", value)
				if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
					t.Errorf("sending %T: err = %v, want invalid argument", value, err)
				}
			}
		})
	}
}
