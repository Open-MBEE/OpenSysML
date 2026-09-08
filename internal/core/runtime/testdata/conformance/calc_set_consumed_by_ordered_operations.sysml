// A set has no order of its own, so an operation that walks its elements in
// order — collect, head, tail, `#`, `==` against a sequence — sees them in the
// canonical order every set enumerates in: Booleans, then numbers ascending,
// then strings, quantities, enumeration literals and objects. The same set
// written in another order answers the same, so the trace of this calc is the
// golden for that order.
package test {
	private import ScalarValues::*;
	private import Collections::*;
	private import ControlFunctions::*;
	private import SequenceFunctions::*;

	attribute s : Set { :>> elements = (30, 10, 20, 20); }

	calc walk {
		attribute scaled : Integer[*] ordered = s.elements->collect{in x; x - 5};
		attribute least : Integer = s.elements->head();
		attribute rest : Integer[*] ordered = s.elements->tail();
		attribute second : Integer = s.elements#(2);
		attribute asWritten : Boolean = s.elements == (30, 10, 20);
		attribute asEnumerated : Boolean = s.elements == (10, 20, 30);
		return : Integer[*] ordered = (scaled#(1), scaled#(2), scaled#(3), least, rest#(1), rest#(2), second, if asWritten ? 1 else 0, if asEnumerated ? 1 else 0);
	}
}
