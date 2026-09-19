// A structured attribute usage names one object of its own, so a chain reads the
// features it redefines and the ones it inherits from its type.
package test {
	private import ScalarValues::*;

	attribute def Cost {
		attribute v : Real default 1.0;
		attribute w : Real default 2.0;
	}

	attribute template : Cost {
		attribute :>> v default 5.0;
	}

	calc def probe {
		return : Real[2] = (template.v, template.w);
	}
}
