// An indexed connection end (an OpenSysML extension) attaches one element of a
// collection-valued port: `s.y#(1)` is the first of the two `P` ports `s.y`
// holds, so the connection typed by `C`, whose ends are single ports, is
// satisfied at an end of multiplicity one and every feature of the assembly,
// the connection's ends included, materializes cleanly.
package test {
	private import ScalarValues::*;

	port def P {
		attribute value : Real default 0.0;
	}

	part def Source {
		port y : P[2];
	}

	part def Sink {
		port u : P;
	}

	connection def C {
		end source[1] : P;
		end target[1] : P;
	}

	part def Asm {
		part s : Source;
		part k : Sink;
		connection link : C connect [1] s.y#(1) to [1] k.u;
	}

	part asm : Asm;
}
