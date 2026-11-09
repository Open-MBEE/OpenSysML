// A constraint redefining a feature of its owner: c6 writes its own value for a,
// the nearer declaration, while c7 inherits the value of b from p.
package test {
	private import ScalarValues::*;

	part p {
		attribute a : Real default 2.0;
		attribute b : Real = 3.0;

		constraint c6 {
			attribute :>> a = 7.0;
			a > 8.0
		}

		constraint c7 {
			attribute :>> b;
			b > 0.0
		}
	}
}
