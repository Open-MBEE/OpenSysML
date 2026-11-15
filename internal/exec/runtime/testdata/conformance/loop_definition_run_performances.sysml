package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	// A definition performed on its own is an occurrence too: the loops its body reaches
	// are its `loops`, and a loop a body iteration reaches is that iteration's.
	action def Run {
		attribute n : Integer = 0;
		attribute m : Integer = 0;
		attribute t : Integer = 0;
		attribute k : Integer = 0;
		out loopCount : Integer;
		out namedBodyCount : Integer;
		out namedBodySteps : Integer;
		out outerBodyCount : Integer;
		out innerLoopCount : Integer;
		out innerBodyCount : Integer;
		out innerBodySteps : Integer;
		out innerStepsAreEnclosed : Boolean;
		out unnamedActionBodySteps : Integer;
		out loopsAreSubactions : Boolean;

		first start;
		then loop action body {
			first start;
			then action inc assign n := n + 1;
			then done;
		} until n >= 3;
		then while m < 2 {
			assign m := m + 1;
			for i in (1, 2) {
				action add assign t := t + i;
			}
		}
		then loop action {
			action bump assign k := k + 1;
		} until k >= 2;
		then assign loopCount := size(loops);
		then assign namedBodyCount := size(loops#(1).body);
		then assign namedBodySteps := size(loops#(1).body.subperformances);
		then assign outerBodyCount := size(loops#(2).body);
		then assign innerLoopCount := size(loops#(2).body.loops);
		then assign innerBodyCount := size(loops#(2).body.loops.body);
		then assign innerBodySteps := size(loops#(2).body.loops.body.subperformances);
		then assign innerStepsAreEnclosed := size(loops#(2).body.loops.body.enclosedPerformances) == innerBodySteps;
		then assign unnamedActionBodySteps := size(loops#(3).body.subperformances);
		then assign loopsAreSubactions := size(subactions) == loopCount;
		then done;
	}
}
