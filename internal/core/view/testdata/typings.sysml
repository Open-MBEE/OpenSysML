package Spelled {
	port def Link;

	part def 'Frame *rail*';

	part def Mount;

	part def Cart;

	connection def Bolt;

	connection def Weld;

	item def Note;

	part def Rig {
		port plug : ~Link;
		port socket : Link;
		part rail : 'Frame *rail*';
		part base typed by Mount, Cart;
		part root : $::Spelled::Mount;
		port mirrored : ~rail.'Frame *rail*';
		connection : Bolt, Weld connect plug to socket;
	}

	occurrence def Exchange {
		part sender {
			event occurrence send;
		}

		part receiver {
			event occurrence receive;
		}

		message : $::Spelled::Note from sender.send to receiver.receive;
	}
}

package SpelledViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view rigView {
		expose Spelled::Rig;
	}

	view rigConnections {
		expose Spelled::Rig;
		render asInterconnectionDiagram;
	}

	view exchangeView : SequenceView {
		expose Spelled::Exchange;
	}
}
