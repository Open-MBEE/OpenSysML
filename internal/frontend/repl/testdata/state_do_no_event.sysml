// A state whose do behavior has actions left with nothing queued, so %advance
// must report them as do work rather than as events processed.
package Debug {
	state Work {
		attribute count = 0;

		entry; then init;
		state init;
		state busy {
			do {
				assign count := count + 1;
				assign count := count + 1;
				assign count := count + 1;
			}
		}

		succession first init then busy;
		succession first busy then done;
	}
}
