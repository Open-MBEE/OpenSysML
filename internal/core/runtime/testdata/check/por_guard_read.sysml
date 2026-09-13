// A write to a feature another branch's succession guard reads: which branch
// the decision takes depends on whether the writer went first.
package test {
	private import ScalarValues::*;

	action gated {
		attribute x : Integer = 0;
		attribute took : String = "none";
		attribute y : Integer = 0;
		first start;
		fork split;
		action writer { assign x := 1; }
		action other { assign y := 1; }
		action lo { assign took := "lo"; }
		action hi { assign took := "hi"; }
		merge chosen;
		join sync;
		done;
		succession first start then split;
		succession first split then writer;
		succession first split then other;
		succession first split then d;
		decide d;
		if x < 1 then lo;
		if x >= 1 then hi;
		succession first writer then sync;
		succession first other then sync;
		succession first lo then chosen;
		succession first hi then chosen;
		succession first chosen then sync;
		succession first sync then done;
	}
}
