package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): two change-triggered
	// transitions out of `watching` rise on the one write of temp, both conditions
	// turning true for temp = 30; the library says one fires, not which. The
	// executor fires the first declared and reports the choice.
	state Monitor {
		attribute temp : Integer = 0;
		attribute route : Integer = 0;

		entry; then start;
		state start;
		state watching;
		state cool {
			entry { assign route := 1; }
		}
		state hot {
			entry { assign route := 2; }
		}

		transition first start do assign temp := 30 then watching;
		transition first watching accept when temp > 20 then cool;
		transition first watching accept when temp > 25 then hot;
	}
}
