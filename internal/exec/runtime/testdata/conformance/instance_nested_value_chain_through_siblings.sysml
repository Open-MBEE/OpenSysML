// A nested item's redefinition bound to a sibling item's feature, itself bound to the
// owner's, reads through the chain: `rf.width = ff.width = Prism::width` is one value seen
// from three features, and a third item reading `rf.width` sees it too. No step of the
// chain is a cycle.
package test {
	private import ScalarValues::Real;

	item def Face {
		attribute length : Real;
		attribute width : Real;
	}

	item def Prism {
		attribute length : Real;
		attribute width : Real;
		item ff : Face {
			attribute :>> length = Prism::length;
			attribute :>> width = Prism::width;
		}
		item rf : Face {
			attribute :>> length = ff.length;
			attribute :>> width = ff.width;
		}
		item edge {
			attribute length : Real = rf.width;
		}
	}

	part wedge : Prism {
		attribute :>> length = 3.0;
		attribute :>> width = 4.0;
	}
}
