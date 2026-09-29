// An action attribute's default evaluating `this` materializes the performance
// occurrence, and a later bare parameter's empty binding stores through the
// occurrence's feature value: the occurrence holds parameters at their
// effective multiplicity (§7.6.3), [0..*] for a bare one, so step reads it
// empty and probe reports size 0.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	action def Seed {
		attribute probe = this;
		in b : Integer[0..*];
		out n : Integer[1];
		first step;
		action step { assign n := b->size(); }
	}
	action probe {
		out n : Integer[1];
		action s : Seed;
		first start;
		then s;
		action read { assign n := s.n; }
		then done;
		succession first s then read;
		succession first read then done;
	}
}
