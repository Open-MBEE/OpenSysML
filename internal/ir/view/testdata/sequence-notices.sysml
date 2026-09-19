// What a sequence rendering cannot show, each of which it reports: an exposed
// element that holds no occurrence, an undirected connector, a message stating
// no ends, and a message reaching outside what the view exposes.
package Kit {
	attribute def Level;
}

package Signals {
	item def Ping;
}

package Talk {
	private import Signals::*;

	occurrence def Handshake {
		part caller {
			port outbox;
			event occurrence sent;
			event occurrence heard;
		}

		part callee {
			port inbox;
			event occurrence taken;
		}

		// A port of the interaction itself, which holds no occurrence and so is
		// no lifeline.
		port monitor {
			event occurrence tapped;
		}

		// Ends naming ports nested in the lifelines.
		flow of Ping from caller.outbox to callee.inbox;
		// Both ends on one lifeline: a self-message.
		message echo of Ping from caller.sent to caller.heard;
		// No direction, so no arrow.
		connect caller.outbox to callee.inbox;
		// No ends at all.
		message pending of Ping;
		// An end reaching nothing the rendering shows.
		message adrift of Ping from caller.sent to monitor.tapped;
	}
}

package NoticeViews {
	private import StandardViewDefinitions::*;

	view handshakeView : SequenceView {
		expose Talk::Handshake;
		expose Kit::Level;
	}
}
