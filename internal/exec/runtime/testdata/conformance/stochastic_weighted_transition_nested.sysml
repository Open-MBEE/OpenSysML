// The weighted pair out of l1 competes for go inside its own region: the
// enclosing work transition on the same event loses to the nested candidates,
// as an unweighted dispatch's would, and the sibling region reacts unweighted.
package test {
	private import ScalarValues::*;
	private import Stochastic::*;
	state def Machine {
		attribute log : Integer = 0;
		entry; then work;
		state work parallel {
			state left {
				entry; then l1;
				state l1;
				state l2 {
					entry { assign log := 10 * log + 2; }
				}
				state l3 {
					entry { assign log := 10 * log + 3; }
				}
				transition first l1 accept go then l2 { @Probability { p = 0.4; } }
				transition first l1 accept go then l3 { @Probability { p = 0.6; } }
			}
			state right {
				entry; then r1;
				state r1;
				state r2 {
					entry { assign log := 10 * log + 1; }
				}
				transition first r1 accept go then r2;
			}
		}
		state escaped;
		transition first work accept go then escaped;
	}
}
