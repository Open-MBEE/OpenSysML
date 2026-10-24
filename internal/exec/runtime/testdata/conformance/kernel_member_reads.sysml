package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	// Kernel Semantic Library members stay out of an object's shape, yet every one
	// of them reads: tracked (`self`, `timeSlices`), declared by the library
	// (`localClock`), inverted through composition (`pa.superoccurrence`), empty
	// where the lower bound is 0 (`predecessors`), undetermined where it is not
	// (`snapshots`). A name the type lacks is refused naming the type.
	part def P1 {
		part pa;
		part pb;
		item ia;
		port pt;
		attribute x : Integer = 1;
		attribute predecessorCount : Integer = size(self.predecessors);
		attribute predecessorsSize : Integer = self.predecessors->size();
		attribute ownPortionCount : Integer = size(timeSlices);
		attribute clockIsUniversal : Boolean = self.localClock == Clocks::universalClock;
	}
}
