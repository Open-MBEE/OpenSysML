// A machine that never settles but advances time each cycle: the timed loop
// between a and b is only ever ended by a budget, never at one instant.
package Debug {
	state Tick {
		entry; then a;
		state a;
		accept after 1 [SI::s] then b;
		state b;
		accept after 1 [SI::s] then a;
	}
}
