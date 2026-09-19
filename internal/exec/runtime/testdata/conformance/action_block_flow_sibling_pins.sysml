package test {
	private import ScalarValues::*;

	// An action declared in a branch of a conditional is a performance of its own,
	// like a node of the action's flow: two siblings in the branch may both declare
	// `v`, a later sibling reads each through the node's name, and the branch's own
	// statements read them the same way. The node not taken is never performed.
	action choose {
		in attribute value : Integer = 5;
		out attribute total : Integer;

		first start;
		then action decide {
			if value > 3 {
				action p { out v : Integer; assign v := value * 2; }
				action q { out v : Integer; assign v := p.v + 1; }
				assign total := p.v + q.v;
			} else {
				action p { out v : Integer; assign v := 0; }
				assign total := p.v;
			}
		}
		then done;
	}
}
