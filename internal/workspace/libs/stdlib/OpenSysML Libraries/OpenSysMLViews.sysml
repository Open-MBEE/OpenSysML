library package OpenSysMLViews {
	doc /*
	 * NON-NORMATIVE rendering vocabulary for case and mixed diagrams.
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
