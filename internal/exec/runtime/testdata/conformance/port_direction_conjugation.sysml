// A message only traverses an end that can receive it: `~Chan` conjugates the
// definition's `out` feature to `in`, so the send arrives at dst.
package PortDirection {
	port def Chan {
		out attribute v : Integer;
	}
	action ship {
		attribute got : Integer = 0;
		port src : Chan;
		port dst : ~Chan;
		connect src to dst;
		first start;
		action sender { send 11 via src; }
		action reader accept n : Integer via dst { assign got := n; }
		done;
		succession first start then sender;
		succession first sender then reader;
		succession first reader then done;
	}
}
