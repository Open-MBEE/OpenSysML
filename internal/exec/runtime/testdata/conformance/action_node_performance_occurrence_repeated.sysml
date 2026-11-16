// A node a merge reaches again is performed again, each performance an occurrence of its
// own: `run.one` lists both in the order begun and `run.one.v` both values (KerML §7.3.4.6).
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
