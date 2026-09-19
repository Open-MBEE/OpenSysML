package test {
	// A calc usage binds its inputs once per evaluation and computes every output
	// from that binding (KerML 7.4.9: an invocation binds its parameters, and its
	// results are the results of that one invocation). Reading `p.a`, changing
	// what `p`'s input named, then reading `p.b` therefore answers `b` from the
	// binding `a` came from — the two outputs can never disagree.
	calc def Pair {
		in k : Real;
		out a = k;
		out b = k * 10.0;
	}

	calc def Interleaved {
		attribute v : Real = 1.0;
		calc p : Pair {
			in k = v;
		}
		attribute ra : Real = p.a;
		assign v := 2.0;
		attribute rb : Real = p.b;
		ra * 1000.0 + rb
	}

	// The same computation with both outputs read before the assignment. Its
	// answer is what Interleaved must also give: the order the outputs are read
	// in is not observable.
	calc def Snapshotted {
		attribute v : Real = 1.0;
		calc p : Pair {
			in k = v;
		}
		attribute ra : Real = p.a;
		attribute rb : Real = p.b;
		assign v := 2.0;
		ra * 1000.0 + rb
	}
}
