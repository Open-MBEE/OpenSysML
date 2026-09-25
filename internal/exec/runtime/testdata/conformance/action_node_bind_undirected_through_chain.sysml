package test {
	private import ScalarValues::*;

	// A binding at a node's undirected attribute whose other end is a feature chain
	// holds at both ends like one at a plain name: the node reads the object's feature
	// as it begins and writes back what it changed as it ends, an unchanged value is
	// left alone, and a `bind` written in an `if` branch at a node the branch declares
	// reaches the object the same way.
	part def Counter {
		attribute count : Integer = 1;
		attribute kept : Integer = 7;
		attribute flag : Integer = 0;
	}

	action outer {
		part counter : Counter;
		out attribute seen : Integer;
		out attribute unchanged : Integer;
		out attribute flagged : Integer;

		bind acc.total = counter.count;
		bind acc.k = counter.kept;

		first start;
		then action acc {
			attribute total : Integer;
			attribute k : Integer;
			assign total := total + 41;
		}
		then action pick {
			if counter.count > 10 {
				bind mark.v = counter.flag;
				action mark { out v : Integer; assign v := 1; }
			}
		}
		then action fin {
			assign seen := counter.count;
			assign unchanged := counter.kept;
			assign flagged := counter.flag;
		}
		then done;
	}
}
