package test {
	private import SequenceFunctions::*;
	// A recursive descent over a structure: each invocation takes one element and
	// hands the rest to itself.
	calc sumFrom {
		in xs : Integer[*];
		in i : Integer;
		return : Integer = if i > xs->size() ? 0 else xs#(i) + sumFrom(xs, i + 1);
	}
	calc total {
		attribute xs : Integer[*] = (5, 7, 9, 11);
		return : Integer = sumFrom(xs, 1);
	}
}
