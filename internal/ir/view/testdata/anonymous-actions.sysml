// A hand-written action flow whose nodes are anonymous: no name, made-up or
// otherwise, so each is labelled by what it does.
package Anonymous {
	private import ScalarValues::*;

	attribute def Go;

	action def Counted {
		attribute n : Integer = 0;
		port inPort;

		action tally { assign n := n + 1; }
		first start then tally;
		then action { assign n := n + 1; }
		then accept Go via inPort;
		then action { out result : Boolean = true; }
		then action { send Go() via inPort; }
		then action { send Go() via inPort; assign n := n + 1; }
		then done;
	}
}

package AnonymousViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view countedView : ActionFlowView {
		expose Anonymous::Counted;
	}
}
