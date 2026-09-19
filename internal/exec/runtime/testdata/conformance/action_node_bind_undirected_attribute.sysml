package test {
	private import ScalarValues::*;

	// A binding at a node's undirected attribute holds at both ends: the node reads the other
	// end as it begins and carries back what it changed (or first gave) as it ends.
	action outer {
		attribute base : Integer = 1;
		attribute kept : Integer = 7;
		attribute fresh : Integer;
		out attribute seen : Integer;
		out attribute unchanged : Integer;
		out attribute given : Integer;
		out attribute downstream : Integer;

		bind acc.total = base;
		bind acc.k = kept;
		bind acc.total = next.n;
		bind giver.v = fresh;

		first start;
		then action acc {
			attribute total : Integer;
			attribute k : Integer;
			assign total := total + 41;
		}
		then action next { attribute n : Integer; assign downstream := n * 2; }
		then action giver { attribute v : Integer; assign v := 9; }
		then action fin {
			assign seen := base;
			assign unchanged := kept;
			assign given := fresh;
		}
		then done;
	}
}
