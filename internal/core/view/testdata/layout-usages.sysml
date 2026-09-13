// Usages typed by the definitions of another document: the states and
// transitions they inherit keep their positions and routes.
package PlantUsages {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	state machine : Plant::Machine;

	view inheritedMachineView : StateTransitionView {
		expose machine;
		metadata Layout about Plant::Machine::off { x = 5; y = 5; }
	}

	// An explicit extent of zero is a size, unlike one left unbound.
	view zeroCanvasView : StateTransitionView {
		expose machine;
		@Canvas { unit = "mm"; width = 0; height = 0; }
	}
}
