// An assignment target may be a feature chain of any depth, in an action body
// and in a state behavior.
package Chains {
	part def Leaf {
		attribute depth : Integer = 0;
	}

	part def Mid {
		part leaf : Leaf;
	}

	part m : Mid;

	action write {
		assign m.leaf := m.leaf;
		assign m.leaf.depth := 3;
	}

	part def Rig {
		part mid : Mid;

		exhibit state run {
			entry; then setting;

			state setting {
				entry action set {
					assign mid.leaf.depth := 4;
				}
			}
		}
	}
}
