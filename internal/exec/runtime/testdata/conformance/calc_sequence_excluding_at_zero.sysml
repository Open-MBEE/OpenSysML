// An index of 0 is no Positive and is refused, as it is for seq#(0).
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def dropAtZero {
		return : Integer[0..*] = (1, 2, 3)->excludingAt(0);
	}
}
