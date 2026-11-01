package InheritedOrderViews {
	private import Views::*;

	part def Engine;

	part def Ends {
		part left : Engine;
		attribute gap;
		part right : Engine;
	}

	// Redefinitions declared in the opposite order keep the redefined members' places.
	part def Swapped :> Ends {
		part :>> right;
		part :>> left;
	}

	part def Rig {
		part swapped : Swapped;
		// An own member declared after a redefinition follows it there.
		part trailed : Ends {
			part :>> right;
			part extra : Engine;
		}
		// An own member declared before any redefinition comes first.
		part led : Ends {
			part extra : Engine;
			part :>> left;
		}
	}

	view rigTree {
		expose Rig;
		render asTreeDiagram;
	}
}
