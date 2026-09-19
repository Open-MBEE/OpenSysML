package model

import "testing"

// F67: inherited-member lookup where the redefinition itself introduces the
// type. `item :>> shape : Box [1] { … }` redefines Items::Item::shape, which
// SpatialItem inherits through its kind's implicit base — the declared chain
// `item def SpatialItem :> SpatialFrame` does not displace Items::Item
// (SysML v2 7.9.3), so `shape` and Box's inherited attributes must resolve.
func TestF67InheritedShapeRedefinedWithType(t *testing.T) {
	src := `package ShapeRedefExample {
		private import SpatialItems::*;
		private import ShapeItems::*;
		private import SI::*;

		part def Engine :> SpatialItem {
			part rawEngineBlock :> subSpatialParts [1] {
				item :>> shape : Box [1] {
					:>> length = 300 [mm];
					:>> width = 190 [mm];
					:>> height = 330 [mm];
				}
			}
		}
	}`
	if got := diagnoseSource(t, "file:///f67_shape_redef.sysml", src); len(got) != 0 {
		t.Fatalf("expected no diagnostics, got %v", got)
	}
}

// The same lookup on a usage typed directly by SpatialItem (the
// SimpleQuadcopter shape), with the redefinition introducing Cylinder.
func TestF67InheritedShapeOnTypedUsage(t *testing.T) {
	src := `package Q {
		private import SI::*;
		private import SpatialItems::*;
		private import ShapeItems::*;

		item motorShape : SpatialItem {
			item :>> shape : Cylinder {
				:>> radius = 18 [mm];
				:>> height = 30 [mm];
			}
		}
	}`
	if got := diagnoseSource(t, "file:///f67_shape_usage.sysml", src); len(got) != 0 {
		t.Fatalf("expected no diagnostics, got %v", got)
	}
}

// The lookup must also carry through nested subsetting of subSpatialParts.
func TestF67InheritedShapeNestedSubsetting(t *testing.T) {
	src := `package Q {
		private import SI::*;
		private import SpatialItems::*;
		private import ShapeItems::*;

		item chassis : SpatialItem {
			part mainBody :> subSpatialParts {
				part rawBody :> subSpatialParts {
					item :>> shape : Box {
						:>> length = 160 [mm];
					}
				}
			}
		}
	}`
	if got := diagnoseSource(t, "file:///f67_shape_nested.sysml", src); len(got) != 0 {
		t.Fatalf("expected no diagnostics, got %v", got)
	}
}

// A bare redefinition of the inherited member, without a type (this minimal
// form already agreed with the pilot before F67 and must keep doing so).
func TestF67InheritedShapeRedefinedBare(t *testing.T) {
	src := `package ShapeRedefExample {
		private import SpatialItems::*;
		private import ShapeItems::*;

		part def Engine :> SpatialItem {
			part rawEngineBlock :> subSpatialParts [1] {
				item :>> shape;
			}
		}
	}`
	if got := diagnoseSource(t, "file:///f67_shape_bare.sysml", src); len(got) != 0 {
		t.Fatalf("expected no diagnostics, got %v", got)
	}
}
