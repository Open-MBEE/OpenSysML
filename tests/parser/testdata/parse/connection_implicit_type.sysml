// A connector usage that names no definition takes its type implicitly from the
// library (SysML v2 §8.3.13), and a `variation interface` declares its
// connections through its variants.
part def Assembly {
	part a {
		port p;
		part inner { port q; }
	}
	part b { port p; }

	interface iface connect a.p to b.p;
	connection untyped connect a.p to b.p;
	connection chained connect a.inner.q to b.p;
	// An allocation usage declares its ends with `allocate`, not `connect`
	// (SysML v2 §8.3.19); a binding uses `bind`, and a flow `flow … to …`.
	allocation alloc allocate a to b;
	binding bnd bind a.p = b.p;
	flow f from a.p to b.p;
	flow from a.p to b.p;
	flow a.p to b.p;

	variation interface link {
		variant interface direct connect a.p to b.p;
		variant interface indirect connect a.inner.q to b.p;
	}
}
