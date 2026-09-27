package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;

	part def Scene;
	action def Focus {
		in scene : Scene;
	}
	action focus : Focus {
		in : Scene;
	}

	calc def Parameters :> Query {
		in root : Element;
		WhereType(source = OwnedElements(source = root), type = "Usage")
	}

	part def ParameterReport :> Document {
		attribute redefines title = "Parameters";
		part parameters : Table {
			calc rows : Parameters { in root = focus; }
		}
	}
}
