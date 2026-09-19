// A redefinition in a usage's body wins over the value its type declares, while
// the features it does not redefine keep the inherited value.
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

	part r : Ring {
		attribute :>> cost {
			attribute :>> v default 11.0;
		}
	}

	calc def probe {
		return : Real[2] = (r.cost.v, r.cost.w);
	}
}
