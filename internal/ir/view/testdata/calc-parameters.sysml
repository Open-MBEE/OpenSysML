// The directed parameters of calculations, actions and constraints: a tree draws them as
// parameter pins of their owner rather than as attribute members, and names the anonymous
// result parameter `result` as the standard names it. A calc usage typed by a calc def shows
// each parameter once: its own redefining one and the def's others as inherited.
package ParameterCalcs {
	private import ScalarValues::*;

	// The reporter's shape: an anonymous result declared by `return`.
	calc add {
		in x : Integer;
		in y : Integer;
		return : Integer;
		x + y
	}

	calc def Add {
		in x : Integer;
		in y : Integer;
		return : Integer = x + y;
	}

	// Every direction, and a named result.
	calc def Scale {
		in x : Real;
		in factor : Real;
		inout count : Integer;
		out doubled : Real = x + x;
		return r : Real = x * factor;
	}

	action def Heat {
		in bread : Integer;
		out toast : Integer;
	}

	constraint def Between {
		in attribute low : Real;
		in attribute value : Real;
		value >= low
	}

	part def Sensor {
		attribute a : Integer;
		attribute b : Integer;
		attribute gain : Real;
		calc sum : Add { in x = a; }
		calc scaled : Scale { in factor = gain; }
	}
}

package ParameterCalcViews {
	private import Views::*;

	view calculations {
		expose ParameterCalcs::add;
		expose ParameterCalcs::Sensor;
		render asTreeDiagram;
	}

	view definitions {
		expose ParameterCalcs::Add;
		expose ParameterCalcs::Scale;
		expose ParameterCalcs::Heat;
		expose ParameterCalcs::Between;
		render asTreeDiagram;
	}
}
