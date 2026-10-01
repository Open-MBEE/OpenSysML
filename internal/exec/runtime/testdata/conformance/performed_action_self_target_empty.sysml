// A `perform action` usage with an empty body names nothing and performs
// itself: the PerformActionUsage is its own event occurrence
// (SysML v2 §8.3.16, eventOccurrence), so the object is created rather than
// refused.
package test {
	part def Idle {
		perform action idle;
	}
}
