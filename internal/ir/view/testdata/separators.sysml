// Names holding the qualified-name separator, so a rendering must quote each
// segment from the owner chain rather than split the joined qualified name.
package 'Sep::Pkg' {
	part def 'Top::Def';

	part 'x::y' {
		port 'fuel::out';
		port sink;
		connection 'fuel::line' connect 'fuel::out' to sink;
	}

	state def 'Run::Mode' {
		entry; then 'idle::state';
		state 'idle::state';
	}
}

package SepViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view partView {
		expose 'Sep::Pkg'::'x::y';
	}

	view lineView {
		expose 'Sep::Pkg'::'x::y';
		render asInterconnectionDiagram;
	}

	view tableView : GridView {
		expose 'Sep::Pkg'::'x::y';
	}

	view modeView : StateTransitionView {
		expose 'Sep::Pkg'::'Run::Mode';
	}
}
