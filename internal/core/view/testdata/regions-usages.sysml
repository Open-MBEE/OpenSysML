// A usage typed by the parallel definition of another document: the regions it
// inherits, top-level and nested, are located where that definition writes them.
package RegionUsages {
	private import Views::*;
	private import StandardViewDefinitions::*;

	state controller : Regions::Controller;

	view controllerView : StateTransitionView {
		expose controller;
	}
}
