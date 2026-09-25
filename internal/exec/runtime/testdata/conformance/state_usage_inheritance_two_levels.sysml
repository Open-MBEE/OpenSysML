// Inheritance composes: a definition whose substate is itself a typed usage.
package test {
	private import ScalarValues::*;

	state def Leaf {
		attribute n : Integer = 0;

		entry; then l1;
		state l1 {
			entry action add {
				assign n := n + 5;
			}
		}
	}

	state def Mid {
		entry; then deep;
		state deep : Leaf;
	}

	state def Top {
		entry; then mid;
		state mid : Mid;
	}
}
