package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): two tokens over `tee then sync`
	// satisfy that one succession, not both, so `sync` waits for `right` each round: the
	// writes alternate right, after, right, after and log = 1212.
	action sameSuccession {
		attribute n : Integer = 0;
		attribute m : Integer = 0;
		attribute log : Integer = 0;

		first start;
		fork split;
		action tick { assign n := n + 1; }
		fork tee;
		decide again;
		action tickEnd;
		action s1;
		action s2;
		action s3;
		action s4;
		action s5;
		action s6;
		action right { assign m := m + 1; assign log := log * 10 + 1; }
		join sync;
		action after { assign log := log * 10 + 2; }
		decide more;
		done;

		succession first start then split;
		succession first split then tick;
		succession first split then s1;
		succession first tick then tee;
		succession first tee then sync;
		succession first tee then again;
		succession first again if n < 2 then tick;
		succession first again if n >= 2 then tickEnd;
		succession first s1 then s2;
		succession first s2 then s3;
		succession first s3 then s4;
		succession first s4 then s5;
		succession first s5 then s6;
		succession first s6 then right;
		succession first right then sync;
		succession first sync then after;
		succession first after then more;
		succession first more if m < 2 then s1;
		succession first more if m >= 2 then done;
	}
}
