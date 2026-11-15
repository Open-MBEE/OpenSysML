// A node a merge reaches again is performed again, and each performance is an
// occurrence of its own: `run.one` lists both in the order they began, so
// the part's `performed = size(run.one)` is 2 and its `values = run.one.v` the two
// values (KerML 1.1 §7.4.9.4 feature chaining), not the latest performance's alone.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part plant {
		attribute values : Integer[0..*] = run.one.v;
		attribute performed : Integer[1] = size(run.one);
		perform action run {
			out attribute total : Integer[1] = 0;
			attribute n : Integer[1] = 0;
			action one { out v : Integer[1]; assign v := n + 1; }
			action tally { assign total := total + one.v; assign n := n + 1; }
			merge again;
			decide more;
			first start then again;
			first again then one;
			first one then tally;
			first tally then more;
			first more if n < 2 then again;
			first more if n >= 2 then done;
		}
	}
}
