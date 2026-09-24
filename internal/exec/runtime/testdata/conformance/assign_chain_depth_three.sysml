// A chain of three segments walks two objects before writing: `m.leaf.depth`
// writes the leaf's feature, not the mid part's, and the node after it reads the
// written value back through the same chain.
package test {
	private import ScalarValues::*;

	part def Leaf {
		attribute depth : Integer = 0;
	}

	part def Mid {
		part leaf : Leaf;
	}

	part m : Mid;

	action write {
		attribute seen : Integer = 0;

		first start;

		action set {
			assign m.leaf.depth := 3;
		}

		action read {
			assign seen := m.leaf.depth;
		}

		done;

		succession first start then set;
		succession first set then read;
		succession first read then done;
	}
}
