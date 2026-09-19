// A library function is denoted by its bare name only where the model imports
// its package, as the checker resolves it: with no import of SequenceFunctions,
// wheels->size() is an unresolved reference at runtime too, naming the packages
// that would provide it.
package test {
	calc wheelCount {
		attribute wheels : ScalarValues::Integer[*] = (1, 2, 3, 4);
		return : ScalarValues::Integer = wheels->size();
	}
}
