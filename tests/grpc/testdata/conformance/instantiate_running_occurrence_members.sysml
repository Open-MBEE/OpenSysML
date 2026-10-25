package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	part def Chain {
		attribute n : Integer = 0;
		perform action a assign n := n + 1;
		perform action b assign n := n + 10;
		first a then b;
		attribute ran : Integer = n;
		attribute bPredecessors : Integer = size(b.predecessors);
		attribute aSuccessors : Integer = size(a.successors);
		attribute aShots : Integer = size(a.snapshots);
		attribute aDone : Boolean = notEmpty(a.done);
		attribute aStarted : Boolean = notEmpty(a.start);
	}
}
