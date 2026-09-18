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
