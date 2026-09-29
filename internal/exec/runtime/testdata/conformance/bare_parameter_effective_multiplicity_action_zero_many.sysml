// A parameter that writes no multiplicity takes the effective multiplicity of
// what it redefines or subsets, else [0..*] (§7.6.3): Count's `in x` is
// optional, so `s` performs with `x` empty and its step reads the size 0, the
// same result the explicit [0..*] variant gives.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	action def Count {
		in x : Integer[0..*];
		out n : Integer;
		first step;
		action step { assign n := x->size(); }
	}

	action probe {
		out n : Integer;
		action s : Count;
		first start;
		then s;
		action read { assign n := s.n; }
		then done;
		succession first s then read;
	}
}
