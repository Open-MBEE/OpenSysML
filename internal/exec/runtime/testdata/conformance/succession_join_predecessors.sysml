package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	part def Join {
		attribute n : Integer = 0;
		perform action a assign n := n + 1;
		perform action c assign n := n + 100;
		perform action b assign n := n + 10;
		first a then b;
		first c then b;
		attribute ran : Integer = n;
		attribute bImmediate : Integer = size(b.immediatePredecessors);
		attribute bAfterA : Boolean = includes(b.immediatePredecessors, a);
		attribute bAfterC : Boolean = includes(b.immediatePredecessors, c);
		attribute aImmediate : Integer = size(a.immediateSuccessors);
		attribute cImmediate : Integer = size(c.immediateSuccessors);
		attribute bBeforeA : Boolean = includes(a.predecessors, b);
		attribute aPrecedesB : Boolean = includes(b.predecessors, a);
		attribute cPrecedesB : Boolean = includes(b.predecessors, c);
		attribute aEndPrecedesB : Boolean = includes(b.predecessors, a.done);
	}
}
