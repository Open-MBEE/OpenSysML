package Calcs {
	private import ScalarValues::*;

	// A calc whose body is only its result expression: its anonymous return
	// parameter is the pin named result.
	calc add {
		in x : Integer;
		in y : Integer;
		return : Integer;
		x + y
	}

	// A calc def binding a named result parameter to its expression.
	calc def Add {
		in x;
		in y;
		return r : Integer = x + y;
	}

	part def Totaliser {
		attribute a : Integer;
		attribute b : Integer;
		// A calc usage typed by Add takes Add's parameters as pins.
		calc total : Add {
			in x = a;
			in y = b;
		}
	}

	// A calc stepping through actions before it yields its result.
	calc def Stepwise {
		in load : Real;
		action measure { out reading : Real; }
		action scale { in raw : Real; out scaled : Real; }
		first measure then scale;
		flow measure.reading to scale.raw;
		return result : Real = scale.scaled;
	}
}

package CalcViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view calcFlows : ActionFlowView {
		expose Calcs::add;
		expose Calcs::Add;
		expose Calcs::Totaliser::total;
		expose Calcs::Stepwise;
		render asInterconnectionDiagram;
	}
}
