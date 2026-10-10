// The nodes of one action body share a feature space (SysML v2 §7.16.5), so an
// accept node's payload is visible by simple name to its sibling nodes — but a
// feature the body itself declares is the nearer declaration (KerML 1.0
// §8.2.3.5.3): `reader`'s `msg` is the body attribute, and the payload is reached
// only through its node, `receiver.msg`.
package test {
	private import ScalarValues::*;

	action communicator {
		attribute msg : Integer = 1;
		attribute seen : Integer = 0;
		attribute payload : Integer = 0;

		first start;
		action sender { send 9 to receiver; }
		action receiver accept msg : Integer;
		action reader {
			assign seen := msg;
			then assign payload := receiver.msg;
		}
		done;

		succession first start then sender;
		succession first sender then receiver;
		succession first receiver then reader;
		succession first reader then done;
	}
}
