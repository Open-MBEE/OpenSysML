package Charging {
	action def Charge {
		first start;
		then merge again;
		then action monitor;
		then decide full;
			if true then supply;
			if false then finish;
		action supply;
		then again;
		action finish;
		then done;
	}
}

package ChargingViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view placed : ActionFlowView {
		expose Charging::Charge;
		render asInterconnectionDiagram;
		metadata Layout about Charging::Charge::start { x = 0; y = 60; }
		metadata Layout about Charging::Charge::again { x = 100; y = 60; }
		metadata Layout about Charging::Charge::monitor { x = 0; y = 160; }
		metadata Layout about Charging::Charge::full { x = 250; y = 160; }
		metadata Layout about Charging::Charge::supply { x = 440; y = 160; }
		metadata Layout about Charging::Charge::finish { x = 250; y = 280; }
		metadata Layout about Charging::Charge::done { x = 0; y = 290; }
	}
}
