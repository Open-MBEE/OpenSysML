package test {
	private import ScalarValues::*;

	// A flow-owning node reached from both branches of a fork is one performance that
	// follows both (docs/project/behavior-semantic-oracle.md): `stage` runs once after
	// `left` and `right` have ended, its two pins holding the one delivery each flow
	// carried, so scaled = collected = (1 + 2) * 10 and `collect` is performed once.
	action pipeline {
		out attribute scaled : Integer = 0;
		out attribute collected : Integer = 0;

		fork split;
		action left { out v : Integer; assign v := 1; }
		action right { out v : Integer; assign v := 2; }
		action stage {
			in x1 : Integer;
			in x2 : Integer;
			out y : Integer;
			action scale { assign y := (x1 + x2) * 10; }
			action tally { assign scaled := scaled + y; }
			first scale;
			then tally;
		}
		action collect { in z : Integer; assign collected := collected + z; }
		join sync;

		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then stage;
		succession first right then stage;
		succession first stage then collect;
		succession first collect then sync;
		succession first sync then done;

		flow left.v to stage.x1;
		flow right.v to stage.x2;
		flow stage.y to collect.z;
	}
}
