package passes

import "testing"

// A metadata body value binds to the feature of the metadata type it restates
// (KerML §7.4.7), so it is checked against that feature's type as any bound value is.
func TestPrefixMetadataBodyValueMustConformToTheRestatedFeature(t *testing.T) {
	wantDiags(t, `package P {
		metadata def Weight { attribute p : ScalarValues::Real; attribute tag : ScalarValues::String; }
		part def Rig {
			attribute w : ScalarValues::Real = 0.5;
			attribute n : ScalarValues::Integer = 1;
			attribute label : ScalarValues::String = "x";
			attribute flag : ScalarValues::Boolean = true;
			part fine { @Weight { p = w; tag = label; } }
			part whole { @Weight { p = n; } }
			part folded { @Weight { p = 1.0 - w; } }
			part worded { @Weight { p = label; } }
			part judged { @Weight { p = flag; } }
			part tagged { @Weight { tag = n; } }
			part literal { @Weight { p = "heavy"; } }
			part widened { @Weight { p = 1; tag = "x"; } }
			part counted { @Weight { tag = 2; } }
		}
	}`,
		"cannot bind String value to a feature typed by Real",
		"cannot bind Boolean value to a feature typed by Real",
		"cannot bind Integer value to a feature typed by String",
		"cannot bind String value to a feature typed by Real",
		"cannot bind Natural value to a feature typed by String")
}

// The `metadata m : M { ... }` form and an explicit `:>>` in the body are checked alike.
func TestMetadataUsageBodyValueMustConformToTheRestatedFeature(t *testing.T) {
	wantDiags(t, `package P {
		metadata def Weight { attribute p : ScalarValues::Real; }
		part def Rig {
			attribute label : ScalarValues::String = "x";
			part fine { metadata weighed : Weight { p = 0.5; } }
			part worded { metadata weighed : Weight { p = label; } }
			part redefined { @Weight { :>> p = label; } }
		}
	}`,
		"cannot bind String value to a feature typed by Real",
		"cannot bind String value to a feature typed by Real")
}

// A body feature typed by its own declaration is judged by that type, as any usage is.
func TestMetadataBodyValueWithItsOwnTypeIsJudgedByIt(t *testing.T) {
	wantDiags(t, `package P {
		metadata def Weight { attribute p : ScalarValues::Real; }
		part def Rig {
			part typed { @Weight { attribute :>> p : ScalarValues::Real = "x"; } }
		}
	}`, "cannot bind String value to a feature typed by Real")
}

// A metadata body value binds as any bound value does: its element count against the
// restated feature's multiplicity and its non-scalar values against its declared type.
func TestMetadataBodyValueIsBoundByTheRestatedFeatureWhole(t *testing.T) {
	wantDiags(t, `package P {
		enum def Level { low; high; }
		part def Wheel;
		metadata def Weight {
			attribute p : ScalarValues::Real[1];
			attribute all : ScalarValues::Real[*];
			attribute level : Level;
			ref part wheel : Wheel;
		}
		part def Rig {
			part fine { @Weight { p = 0.5; all = (0.3, 0.7); level = Level::high; } }
			part pair { @Weight { p = (0.3, 0.7); } }
			part redefined { @Weight { :>> p = (0.3, 0.7); } }
			part restated { metadata weighed : Weight { p = (0.3, 0.7); } }
			part twice { @Weight { all = (0.3, 0.3); } }
			part leveled { @Weight { level = 3; } }
			part wheeled { @Weight { wheel = 3; } }
		}
	}`,
		"2 value(s) bound to a feature with multiplicity upper bound 1",
		"2 value(s) bound to a feature with multiplicity upper bound 1",
		"2 value(s) bound to a feature with multiplicity upper bound 1",
		"0.3 (a Real) is written at positions 1 and 2 of a unique feature",
		"cannot bind Natural value to a feature typed by Level",
		"cannot bind Natural value to a feature typed by Wheel")
}
