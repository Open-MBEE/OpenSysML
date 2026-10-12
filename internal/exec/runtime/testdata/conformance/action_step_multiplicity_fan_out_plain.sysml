package test {
	private import ScalarValues::*;

	// A plain `then` out of a repeated step does not say whether one performance of
	// its target follows every performance of a or one each; fanning out to two
	// targets leaves each edge as open as one does, so the order is refused.
	action def FanOutPlain {
		attribute sum : Integer = 0;
		action a[3] { assign sum := sum + 1; }
		action q;
		action r;
		succession first start then a;
		succession first a then q;
		succession first a then r;
	}
}
