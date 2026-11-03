package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def drop {
		in x : Integer;
		return : Integer[0..*] nonunique = ((1, 2, 3)->excludingAt(x), (1, 2, 3)->excludingAt(x, x + 1));
	}
}
