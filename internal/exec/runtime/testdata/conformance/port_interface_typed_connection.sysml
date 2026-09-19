// An interface usage joins its ends the way a connection does, so a message
// routes over an interface-typed connection between conjugate ports.
package InterfaceConnection {
	port def Chan {
		out attribute v : Integer;
	}
	interface def Link {
		end port source : Chan;
		end port sink : ~Chan;
	}
	action ship {
		attribute got : Integer = 0;
		port src : Chan;
		port dst : ~Chan;
		interface link : Link connect src to dst;
		first start;
		action sender { send 13 via src; }
		action reader accept n : Integer via dst { assign got := n; }
		done;
		succession first start then sender;
		succession first sender then reader;
		succession first reader then done;
	}
}
