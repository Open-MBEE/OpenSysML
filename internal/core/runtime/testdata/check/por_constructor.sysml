// Constructor: one branch creates an object, another counts the extent of its
// type. The count depends on the order of the two moves, though the constructor
// names no feature the count reads. Each branch takes a further step before the
// join so the two are not already dependent through it.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part def Sensor;

	action populate {
		attribute made : Sensor;
		attribute count : Integer = 0;
		attribute builtMark : Integer = 0;
		attribute talliedMark : Integer = 0;
		first start;
		fork split;
		action build { assign made := new Sensor(); }
		action built { assign builtMark := 1; }
		action tally { assign count := size(all Sensor); }
		action tallied { assign talliedMark := 1; }
		join sync;
		done;
		succession first start then split;
		succession first split then build;
		succession first split then tally;
		succession first build then built;
		succession first tally then tallied;
		succession first built then sync;
		succession first tallied then sync;
		succession first sync then done;
	}
}
