// A receiver addressed by a qualified name is the element that name resolves to,
// not a path through the sender's features: `::` separates namespaces where `.`
// chains features, so the machine reaches `done` on the signal it sent itself.
package SendIdentityQualified {
	item def Ping;

	part def Node {
		state Machine {
			entry; then start;
			state start;

			state waiting {
				entry send Ping to SendIdentityQualified::Node::Machine;
			}

			succession first start then waiting;
			transition first waiting when Ping then done;
		}
	}

	part solo : Node;
}
