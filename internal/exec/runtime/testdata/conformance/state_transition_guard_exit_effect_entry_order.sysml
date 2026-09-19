package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): guard (x = 5), then
	// exit (x := 0), then effect (y := x), then the target's entry (z := y + 1).
	state Ordered {
		attribute x : Integer = 5;
		attribute y : Integer = -1;
		attribute z : Integer = -1;

		entry; then active;
		state active {
			exit { assign x := 0; }
		}
		state finished {
			entry { assign z := y + 1; }
		}

		transition first active accept Go if x > 0 do assign y := x then finished;
	}
}
