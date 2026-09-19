// A nested port routes as the path it was written with, so `p.q` reaches the end
// joined to `p.q` and not the same-named port of another port.
package NestedPort {
	action ship {
		attribute got : Integer = 0;
		port p { port q; }
		port other { port q; }
		port sink;
		connect p.q to sink;
		first start;
		action sender { send 7 via p.q; }
		action reader accept n : Integer via sink { assign got := n; }
		done;
		succession first start then sender;
		succession first sender then reader;
		succession first reader then done;
	}
}
