// A loop a case body reaches is a performance held from then on, so an assertion ordered around
// it (§7.20.3) reads the loops held at its position; `in` binds only as stated (KerML §8.2.3.5).
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	analysis def Tally {
		attribute n : Integer := 0;
		action start { assign n := 0; }
		then assert constraint unlooped { in held : Integer = size(loops); held == 0 }
		then loop action body {
			action inc assign n := n + 1;
		} until n >= 3;
		then assert constraint iterated { in rounds : Integer = size(loops.body); rounds == 3 and n == 3 }
		then assert constraint stepped { in steps : Integer = size(loops.body.subperformances); steps == 3 }
		then assert constraint early { in count : Integer = n; count == 0 }
		then action reset { assign n := 0; }
		assert constraint cleared { in count : Integer = n; count == 0 }
		assert constraint kept { in rounds : Integer = size(loops.body); rounds == 3 }
		return shape : Integer = size(loops) * 100 + size(loops.body) * 10 + size(loops.body.subperformances);
	}

	// The usage inherits the flow, the loop and each assertion at its position.
	analysis tally : Tally;
}
