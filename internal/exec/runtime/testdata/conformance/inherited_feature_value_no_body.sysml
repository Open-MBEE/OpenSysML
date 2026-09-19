// A feature valued by a structured attribute usage with no body of its own reads
// that usage's own features, and the ones it inherits from its type.
package test {
	private import ScalarValues::*;

	attribute def Cost {
		attribute v : Real default 1.0;
		attribute w : Real default 2.0;
	}

	attribute template : Cost {
		attribute :>> v default 5.0;
	}

	part def Ring {
		attribute cost : Cost default template;
	}

	part plain : Ring;

	calc def probe {
		return : Real[2] = (plain.cost.v, plain.cost.w);
	}
}
