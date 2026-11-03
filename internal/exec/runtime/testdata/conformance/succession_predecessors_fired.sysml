package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	part def Chain {
		attribute n : Integer = 0;
		perform action a assign n := n + 1;
		perform action b assign n := n + 10;
		perform action c assign n := n + 100;
		first a then b;
		first b then c;
		attribute ran : Integer = n;
		attribute aAfterOwnStart : Boolean = includes(a.predecessors, self.start);
		attribute ownStartBeforeA : Boolean = includes(self.start.successors, a);
		attribute aPredecessors : Integer = size(a.predecessors);
		attribute bPredecessors : Integer = size(b.predecessors);
		attribute cPredecessors : Integer = size(c.predecessors);
		attribute cImmediate : Integer = size(c.immediatePredecessors);
		attribute aSuccessors : Integer = size(a.successors);
		attribute aImmediate : Integer = size(a.immediateSuccessors);
		attribute bAfterA : Boolean = includes(b.predecessors, a);
		attribute cAfterA : Boolean = includes(c.predecessors, a);
	}
}
