package Kit {
	part def Widget;
}

package ErrorViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	// A view exposing nothing: a plain import exposes nothing.
	view emptyView {
		private import Kit::*;
	}

	// A geometry view: the notation is understood, the rendering is not produced.
	view geometryView : GeometryView {
		expose Kit::Widget;
	}

	// A sequence view exposing nothing: an empty sequence diagram.
	view emptySequenceView : SequenceView {
		private import Kit::*;
	}

	// A state view exposing nothing: an empty state diagram.
	view emptyStateView : StateTransitionView {
		private import Kit::*;
	}

	// A structural element an action rendering cannot show.
	view misfitView : ActionFlowView {
		expose Kit::Widget;
	}
}
