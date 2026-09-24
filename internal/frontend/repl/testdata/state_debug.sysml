// A packaged state machine with timed transitions, so %advance has events
// scheduled at distinct times to drain.
package Debug {
	state Cycle {
		entry; then init;
		state init;
		state waiting;
		accept after 10 [SI::s] then working;
		state working;
		accept after 5 [SI::s] then done;

		succession first init then waiting;
	}
}
