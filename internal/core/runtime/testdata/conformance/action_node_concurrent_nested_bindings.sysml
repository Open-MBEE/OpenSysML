package test {
	private import ScalarValues::*;

	// `leg`, reached from both branches of a fork, is one performance that follows both
	// (docs/project/behavior-semantic-oracle.md). A binding between two of its nested pins
	// (`leg.inner.v = leg.rest.n`) and one to its own pin (`leg.inner.w = leg.x1`) hold
	// within that performance: inner scales left's 1, rest multiplies by right's 2 → 20.
	action pipeline {
		out attribute collected : Integer = 0;

		bind leg.inner.w = leg.x1;
		bind leg.inner.v = leg.rest.n;

		fork split;
		action left { out v : Integer; assign v := 1; }
		action right { out v : Integer; assign v := 2; }
		action leg {
			in x1 : Integer;
			in x2 : Integer;
			action inner { in w : Integer; out v : Integer; assign v := w * 10; }
			action rest { in n : Integer; assign collected := collected + n * x2; }
			first inner;
			then rest;
		}
		join sync;

		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then leg;
		succession first right then leg;
		succession first leg then sync;
		succession first sync then done;

		flow left.v to leg.x1;
		flow right.v to leg.x2;
	}
}
