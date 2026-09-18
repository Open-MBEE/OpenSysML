// Notation the Open-MBEE corpus models write, which every validation tier must
// accept: a conjugated end on an interface definition and on a connection
// definition (spacecraft-example-model.sysml), and an occurrence portion whose
// kind keyword follows the portion prefix, named as its whole occurrence is
// (Dragon.sysml `item item1 { timeslice item item1; }`).
package CorpusNotation {
	item def Data;
	item def Ping;

	port def CommunicationPort {
		out item data : Data;
		in item ping : Ping;
	}

	interface def CommunicationInterface {
		end groundPort : CommunicationPort;
		end spacePort : ~CommunicationPort;
		connect spacePort.data to groundPort.data;
		flow from groundPort.ping to spacePort.ping;
	}

	connection def SpaceLink {
		end groundPort : CommunicationPort;
		end spacePort : ~CommunicationPort;
	}

	part def GroundStation {
		port commPort : ~CommunicationPort;
	}

	item item1 {
		attribute attribute1;
		timeslice item item1;
		snapshot item item1;
	}
}
