// A nested item's redefinition bound to itself — `rf.width` written inside `rf`, where
// `rf` names the item being declared — is a cyclic feature value: the read reports the
// cycle naming the feature by the objects walked to it, and the check `-instantiate` makes
// reports that one failure once, not again for each feature that reads through it.
package test {
	private import ScalarValues::Real;

	item def Face {
		attribute width : Real;
		attribute depth : Real;
	}

	item def Prism {
		attribute width : Real;
		item rf : Face {
			attribute :>> width = rf.width;
			attribute :>> depth = width;
		}
		item edge {
			attribute length : Real = rf.width;
		}
	}

	part wedge : Prism {
		attribute :>> width = 4.0;
	}
}
