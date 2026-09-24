package Budget {
	private import DiagramLayout::*;
	private import Views::*;

	metadata def Approved {
		attribute by : ScalarValues::String;
	}

	part def Mirror {
		attribute errorReq : ScalarValues::Real;
		attribute errorCBE : ScalarValues::Real;
		// A user annotation is model content the tree shows.
		metadata Approved about errorCBE { by = "review"; }
		// Layout stated inline is about a picture, not the part.
		@Layout { x = 21; y = 56; width = 328; height = 108; }
		metadata Layout about errorReq { x = 24; y = 108; width = 322; height = 14; }
		metadata placed : Layout about errorCBE { x = 24; y = 122; width = 322; height = 14; }

		view details {
			expose errorReq;
			@Canvas { unit = "px"; width = 1006; height = 570; }
			metadata Layout about errorReq { x = 1; y = 2; }
			render asTreeDiagram;
		}
	}

	// A rendering usage declared outside a view is an ordinary member.
	rendering asTree : GraphicalRendering;
}

package BudgetViews {
	private import Views::*;

	view budgetView {
		expose Budget;
		render asTreeDiagram;
	}
}
