package Spelled {
	port def Link;

	part def 'Frame *rail*';

	part def Mount;

	part def Cart;

	connection def Bolt { end : Link; end : Link; }

	connection def Weld { end : Link; end : Link; }

	alias Note for Transfers::MessageTransfer;

	part def Rig {
		port plug : ~Link;
		port socket : Link;
		part rail : 'Frame *rail*';
		part base defined by Mount, Cart;
		part root : $::Spelled::Mount;
		connection : Bolt, Weld connect plug to socket;
	}

	occurrence def Exchange {
		part sender {
			event occurrence emit;
		}

		part receiver {
			event occurrence receive;
		}

		message : $::Spelled::Note from sender.emit to receiver.receive;
	}
}

package Sites {
	part def Control;

	part def 'Summit Installation' {
		part computer : Control;
	}
}

package Physical {
	private import Sites::*;

	part def 'APS Physical' {
		part 'summit Installation' : 'Summit Installation';
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

	view siteView {
		expose Physical::'APS Physical'::'summit Installation';
		expose Sites::'Summit Installation'::computer;
		render asInterconnectionDiagram;
	}
}
