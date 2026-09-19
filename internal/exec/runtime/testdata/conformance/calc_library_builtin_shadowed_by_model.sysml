// A model's own calc keeps its body where its qualified name is also a built-in
// function's: resolution decides which declaration a call names, and the
// built-in answers only for the library's declaration.
package NumericalFunctions {
	private import ScalarValues::*;

	calc def sum0 {
		in collection : Integer[*];
		in zero : Integer;
		return : Integer = 42;
	}
}

package test {
	private import ScalarValues::*;

	calc def size { in n : Integer; return : Integer = n * 10; }

	calc def Probes {
		out ownByPosition = NumericalFunctions::sum0((1, 2, 3), 0);
		out ownByName = NumericalFunctions::sum0(zero = 0, collection = (1, 2, 3));
		out ownLocal = size(4);
		out librarySize = SequenceFunctions::size((1, 2, 3));
	}

	calc p : Probes;
}
