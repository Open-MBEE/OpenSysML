// Sending into an end whose flow features only carry outward is a typed error:
// `src` cannot receive what `dst` sends it.
package OutboundOnlySend {
	port def Chan {
		out attribute v : Integer;
	}
	action ship {
		port src : Chan;
		port dst : ~Chan;
		connect src to dst;
		first start;
		action sender { send 11 via dst; }
		done;
		succession first start then sender;
		succession first sender then done;
	}
}
