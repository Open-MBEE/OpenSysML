package Chain {
	// A containment chain deeper than a test's lowered tree bound.
	part a {
		part b {
			part c {
				part d;
			}
		}
	}
}

package ChainViews {
	private import Views::*;

	// Exposes the whole chain; a bound that cuts a's tree short of d must not
	// suppress d along with the containers a's surviving tree draws.
	view deepView {
		render asTreeDiagram;
		expose Chain::a;
		expose Chain::a::b;
		expose Chain::a::b::c;
		expose Chain::a::b::c::d;
	}
}
