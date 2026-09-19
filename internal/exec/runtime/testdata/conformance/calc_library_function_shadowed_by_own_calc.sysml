// A calc the model declares outranks the imported library function of the same
// name, as an owned member outranks an imported one in name resolution: with
// SequenceFunctions imported, wheels->size() and size(wheels) are both test::size.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def size { in xs : Integer[*]; return : Integer = 42; }

	calc wheelCount {
		attribute wheels : Integer[*] = (1, 2, 3, 4);
		return : Integer = wheels->size() + size(wheels);
	}
}
