// Orders stated over the events messages run between: one interaction the
// successions reorder away from declaration order, and one whose successions
// contradict each other.
package Signals {
	item def Tick;
}

package Ordering {
	private import Signals::*;

	occurrence def Relay {
		part left {
			event occurrence lateSent;
			event occurrence earlySent;
		}

		part right {
			event occurrence lateTaken;
			event occurrence earlyTaken;
		}

		// Declared first, but the succession below puts it last.
		message late of Tick from left.lateSent to right.lateTaken;
		message early of Tick from left.earlySent to right.earlyTaken;

		succession first right.earlyTaken then left.lateSent;
	}

	occurrence def Deadlock {
		part near {
			event occurrence sent;
			event occurrence heard;
		}

		part far {
			event occurrence taken;
			event occurrence spoke;
		}

		message ask of Tick from near.sent to far.taken;
		message reply of Tick from far.spoke to near.heard;

		succession answering first far.taken then far.spoke;
		succession first near.heard then near.sent;
	}
}

package OrderingViews {
	private import StandardViewDefinitions::*;

	view relayView : SequenceView {
		expose Ordering::Relay;
	}

	view deadlockView : sv {
		expose Ordering::Deadlock;
	}
}
