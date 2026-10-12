package MixedInheritedActor {
	private import OpenSysMLRenderings::*;

	part def Person;
	part def Vehicle;

	use case def Base {
		actor driver : Person;
		subject vehicle : Vehicle;
	}

	use case def A :> Base;
	use case def B :> Base;

	view mixedView {
		expose A;
		expose B;
		expose Person;
		expose Vehicle;
		render asMixedDiagram;
	}

	view caseView {
		expose A;
		expose B;
		render asCaseDiagram;
	}
}
