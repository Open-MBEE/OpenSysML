// A usage typed by the parallel definition of another document: the regions it
// inherits, top-level and nested, are located where that definition writes them.
package RegionUsages {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	state controller : Regions::Controller;

	view controllerView : StateTransitionView {
		expose controller;
		metadata Layout about Regions::Controller::pick { x = 15; y = 25; }
	}
}
