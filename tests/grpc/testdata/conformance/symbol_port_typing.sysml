// A port typed `~CmdPort` reports its type with the `~` the notation wrote
// (SysML v2 8.3.12.3 ConjugatedPortTyping), apart from a port typed `CmdPort`.
package ConjugatedPorts {
	port def CmdPort {
		out attribute cmd : ScalarValues::Integer;
	}

	part def Sender {
		port cmdOut : CmdPort;
	}

	part def Receiver {
		port cmdIn : ~CmdPort;
	}
}
