package opensysml_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// A frame is the object it was read from when it has one, else what it measures,
// so one cut short at a cycle or read under another name is still itself.
func TestCoordinateFramesCompareAsTheServiceDoes(t *testing.T) {
	mm := opensysml.MeasurementRef{Unit: "mm", UnitID: "SI::millimetre"}
	s := opensysml.MeasurementRef{Unit: "s", UnitID: "SI::second"}
	datum := &opensysml.CoordinateFrame{Name: "datum", FrameID: "F::datum", InstanceID: 3, Dimensions: []int64{1}, Axes: []opensysml.MeasurementRef{mm}}
	own := &opensysml.CoordinateTransformation{
		Name: "placed", InstanceID: 9, Source: datum,
		Placement: &opensysml.FramePlacement{Origin: opensysml.Real(1)},
		Pose:      &opensysml.FramePose{Origin: []float64{1}, Basis: []float64{1}},
	}
	posed := &opensysml.CoordinateFrame{Name: "posed", FrameID: "F::posed", InstanceID: 7, Dimensions: []int64{1}, Axes: []opensysml.MeasurementRef{mm}, Transformation: own}
	cut := &opensysml.CoordinateFrame{Name: "posed", FrameID: "F::posed", InstanceID: 7, Dimensions: []int64{1}, Axes: []opensysml.MeasurementRef{mm}}
	other := &opensysml.CoordinateFrame{Name: "posed", FrameID: "F::posed", InstanceID: 8, Dimensions: []int64{1}, Axes: []opensysml.MeasurementRef{mm}, Transformation: own}
	composed := &opensysml.CoordinateFrame{Name: "datum / s", Dimensions: []int64{1}, Axes: []opensysml.MeasurementRef{mm}}
	renamed := &opensysml.CoordinateFrame{Name: "rate", FrameID: "F::rate", TypeID: "F::Rate", Dimensions: []int64{1}, Axes: []opensysml.MeasurementRef{mm}}
	overSeconds := &opensysml.CoordinateFrame{Name: "datum / s", Dimensions: []int64{1}, Axes: []opensysml.MeasurementRef{s}}

	direct := *own
	direct.Name, direct.Target, direct.Pose = "trs", cut, nil
	constructed := &opensysml.CoordinateTransformation{Name: "a", Source: datum, Placement: own.Placement}
	again := &opensysml.CoordinateTransformation{Name: "b", TypeID: "F::P", Source: datum, Placement: own.Placement, Pose: own.Pose}
	moved := &opensysml.CoordinateTransformation{Name: "a", Source: datum, Placement: &opensysml.FramePlacement{Origin: opensysml.Real(2)}}
	shapeless := &opensysml.CoordinateTransformation{Name: "a", InstanceID: 4, Source: datum}
	shapelessToo := &opensysml.CoordinateTransformation{Name: "a", InstanceID: 5, Source: datum}

	for _, c := range []struct {
		name string
		a, b opensysml.Value
		want bool
	}{
		{"a frame cut short at a cycle", posed, cut, true},
		{"two objects of equal content", posed, other, false},
		{"a composed frame under another name", composed, renamed, true},
		{"composed frames over other axes", composed, overSeconds, false},
		{"a composed frame and an object", composed, datum, false},
		{"one transformation with and without its target", own, &direct, true},
		{"constructed transformations of equal content", constructed, again, true},
		{"transformations placing elsewhere", constructed, moved, false},
		{"shapeless transformations of two objects", shapeless, shapelessToo, false},
		{"two usages given one object id by two answers", posed, &opensysml.CoordinateFrame{Name: "datum", FrameID: "F::datum", InstanceID: 7, Dimensions: []int64{1}, Axes: []opensysml.MeasurementRef{mm}}, false},
		{"a shapeless transformation read twice", shapeless, &opensysml.CoordinateTransformation{Name: "a", InstanceID: 4, Source: datum, Target: cut}, true},
		{"a frame and a transformation", posed, own, false},
	} {
		if got := opensysml.Equal(c.a, c.b); got != c.want {
			t.Errorf("%s: Equal = %v, want %v", c.name, got, c.want)
		}
		if got := opensysml.Equal(c.b, c.a); got != c.want {
			t.Errorf("%s, reversed: Equal = %v, want %v", c.name, got, c.want)
		}
	}
	if !(opensysml.Set{posed}).Contains(cut) {
		t.Errorf("a set of a frame does not contain the same frame cut short")
	}
}
