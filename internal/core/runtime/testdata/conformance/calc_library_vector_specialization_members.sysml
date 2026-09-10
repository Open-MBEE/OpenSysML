// A NumericalVectorValue specialization's own members stay readable on the vector
// its object is: read directly, through a calc parameter, and from a calc's result.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;

	attribute def Tagged :> NumericalVectorValue {
		attribute tag : String default "v";
	}

	attribute t : Tagged {
		:>> dimension = 2;
		:>> elements = (1, 2);
	}

	attribute named : Tagged {
		:>> dimension = 2;
		:>> elements = (3, 4);
		:>> tag = "named";
	}

	calc def tagOf {
		in x : Tagged;
		return : String = x.tag;
	}

	calc def pass {
		in x : Tagged;
		return : Tagged = x;
	}

	calc def tags {
		return : String[3] nonunique = (t.tag, tagOf(t), pass(named).tag);
	}
}
