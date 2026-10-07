library package OpenSysMLRenderings {
	doc /*
	 * NON-NORMATIVE rendering vocabulary for case and mixed diagrams.
	 * Models using it are valid SysML v2 and depend on this OpenSysML library.
	*/

	private import Views::*;

	rendering asCaseDiagram : GraphicalRendering[1] :> renderings;
	rendering asMixedDiagram : GraphicalRendering[1] :> renderings;

	view def CaseView {
		render asCaseDiagram;
	}

	view def MixedView {
		render asMixedDiagram;
	}
}
