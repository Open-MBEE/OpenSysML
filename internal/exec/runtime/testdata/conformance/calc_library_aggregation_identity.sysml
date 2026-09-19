// sum0 and product1 fold a collection and answer the identity they are given
// for an empty one, in the kind of the elements or of that identity.
package test {
	private import ScalarValues::*;
	private import NumericalFunctions::*;

	calc def Aggregations {
		out total = NumericalFunctions::sum0((1, 2, 3), 0);
		out realTotal = NumericalFunctions::sum0((1.5, 2), 0);
		out emptyTotal = NumericalFunctions::sum0((), 0);
		out emptyRealTotal = NumericalFunctions::sum0((), 0.0);
		out product = NumericalFunctions::product1((2, 3, 4), 1);
		out emptyProduct = NumericalFunctions::product1((), 1);
		out unqualifiedTotal = sum0((4, 5), 0);
	}

	calc c : Aggregations;
}
