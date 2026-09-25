// A finite chain of untriggered transitions: every event is at one instant,
// but a large enough budget reaches done.
package Debug {
	state Chain {
		entry; then a;
		state a;
		state b;
		state c;
		transition first a then b;
		transition first b then c;
		transition first c then done;
	}
}
