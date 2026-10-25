// An attribute a behavior declares over its own occurrence features reads the
// performance's features, not the performer's: `entryAction`, `doAction` and
// `exitAction`, a declared `en`, `subactions`, `subperformances` and
// `enclosedPerformances` named in a state definition's body are the exhibited
// machine's, and `subactions` in an action definition's body is the performed
// action's. The counts hold in the live run and in a held-image copy of it.
package test {
	private import ScalarValues::Integer;
	private import SequenceFunctions::size;

	state def Modes {
		entry action en {}
		then idle;
		state idle;
		attribute entryCount : Integer = size(entryAction);
		attribute enCount : Integer = size(en);
		attribute doCount : Integer = size(doAction);
		attribute exitCount : Integer = size(exitAction);
		attribute subactionCount : Integer = size(subactions);
		attribute subperformanceCount : Integer = size(subperformances);
		attribute enclosedCount : Integer = size(enclosedPerformances);
	}
	action def Work {
		action a1 {}
		action a2 {}
		attribute subactionCount : Integer = size(subactions);
	}
	part def Machine {
		exhibit state modes : Modes;
		perform action work : Work;
	}
}
