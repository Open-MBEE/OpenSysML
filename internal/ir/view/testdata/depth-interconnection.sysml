package DepthLinks {
	private import Views::*;

	port def Signal;

	part system {
		part plant {
			port status : Signal;
			part pump {
				port 'out' : Signal;
				part motor {
					part rotor;
				}
			}
			part tank {
				port 'in' : Signal;
			}
			connect pump.out to tank.in;
		}
		part controller {
			port cmd : Signal;
			port monitor : Signal;
		}
		// Crosses the cut: pump lies below the depth bound, so the connector ends
		// at plant, the drawn element standing in for it.
		connect controller.cmd to plant.pump.out;
		connect controller.monitor to plant.status;
	}

	view links {
		expose system;
		attribute depth = 1;
		render asInterconnectionDiagram;
	}
}
