// An object's state machine addresses a receiving node of an action it performs.
// The performance is no object of its own, so object identity cannot tell it
// apart from the sender: the accept inside it takes the message and the machine
// leaves `sending`, rather than confining the message to the sending object.
package SendIdentityNested {
	action def Listen {
		first start;

		action reader accept n : Integer;

		done;

		succession first start then reader;
		succession first reader then done;
	}

	part def Node {
		state Machine {
			entry; then start;
			state start;

			state sending {
				entry send 7 to reader;
			}

			succession first start then sending;
			transition first sending do perform Listen then done;
		}
	}

	part solo : Node;
}
