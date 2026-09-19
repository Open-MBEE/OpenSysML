// The model of calc_library_function_unimported with the one import it lacked:
// SequenceFunctions::* makes size a name the calc's scope resolves, so
// wheels->size() is SequenceFunctions::size and counts the wheels.
package test {
	private import SequenceFunctions::*;

	calc wheelCount {
		attribute wheels : ScalarValues::Integer[*] = (1, 2, 3, 4);
		return : ScalarValues::Integer = wheels->size();
	}
}
