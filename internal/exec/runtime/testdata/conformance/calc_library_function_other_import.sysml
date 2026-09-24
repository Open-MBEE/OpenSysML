// An import brings in only the package it names: ScalarValues::* gives the calc
// its Integer type and nothing of SequenceFunctions, so size stays unresolved.
package test {
	private import ScalarValues::*;

	calc wheelCount {
		attribute wheels : Integer[*] = (1, 2, 3, 4);
		return : Integer = wheels->size();
	}
}
