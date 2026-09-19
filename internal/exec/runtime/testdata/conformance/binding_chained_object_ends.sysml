// Bindings chained through two levels of assembly join three ports into one
// object, whichever end is read first: the outer boundary port is materialized
// before the inner assembly's, and the leaf's port last.
package test {
	port def PP;

	part def Leaf {
		port i : ~PP;
	}

	part def InnerAssembly {
		port bi : ~PP;
		part leaf : Leaf;

		bind bi = leaf.i;
	}

	part def OuterAssembly {
		port bi : ~PP;
		part inner : InnerAssembly;

		bind bi = inner.bi;
	}
}
