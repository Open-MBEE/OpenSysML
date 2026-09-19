package grpc

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const libraryFeatureValueModel = `
package Demo {
  private import ShapeItems::*;
  private import SI::*;
  part box : Box {
    :>> length = 2 [m];
    :>> width = 1 [m];
    :>> height = 1 [m];
  }
}
`

// The Systems and Domain library features an object inherits are sent like its
// own: a derived boolean as a value, an empty collection as no values, a
// valueless optional feature unset, and the Kernel frame not at all.
func TestInstantiate_SendsInheritedLibraryFeatureValues(t *testing.T) {
	resp := instantiate(t, libraryFeatureValueModel, "library-feature-values", "Demo::box")
	fvs := resp.GetInstance().GetFeatureValues()

	isSolid := fvs["isSolid"]
	if isSolid == nil {
		t.Fatalf("feature values %v carry no isSolid", featureNames(fvs))
	}
	if !isSolid.GetValue().GetBoolValue() || isSolid.GetError() != "" {
		kind, value := describeValue(isSolid.GetValue())
		t.Errorf("isSolid: %s %v (error %q), want true", kind, value, isSolid.GetError())
	}

	voids := fvs["voids"]
	if voids == nil {
		t.Fatalf("feature values %v carry no voids", featureNames(fvs))
	}
	if voids.GetValue() != nil || len(voids.GetValues()) != 0 || voids.GetError() != "" {
		t.Errorf("voids: value %v values %v error %q, want an empty collection",
			voids.GetValue(), voids.GetValues(), voids.GetError())
	}

	shape := fvs["shape"]
	if shape == nil {
		t.Fatalf("feature values %v carry no shape", featureNames(fvs))
	}
	if _, ok := shape.GetValue().GetKind().(*pb.Value_Unset); !ok {
		kind, value := describeValue(shape.GetValue())
		t.Errorf("shape: %s %v, want unset", kind, value)
	}

	if fvs["length"].GetValue().GetKind() == nil {
		t.Errorf("length: no value, want the redefined 2 [m]")
	}
	for _, frame := range []string{"self", "portions", "timeSlices", "snapshots", "startShot"} {
		if _, ok := fvs[frame]; ok {
			t.Errorf("feature values carry Kernel frame feature %q", frame)
		}
	}
}

func featureNames(fvs map[string]*pb.FeatureValue) []string {
	names := make([]string, 0, len(fvs))
	for name := range fvs {
		names = append(names, name)
	}
	return names
}
