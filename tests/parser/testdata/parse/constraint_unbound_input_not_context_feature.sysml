// An `in` parameter of a constraint usage is the feature its body names (KerML
// §8.2.3.5), and a feature of the owning part spelled as it is does not bind it:
// only a binding the usage states does, while a constraint declaring no such
// parameter reads the features of its context directly (SysML v2 §7.20.1). So
// `masked`, whose input nothing binds, is undetermined naming the input, itself
// and through its members, where `direct` and `bound` decide over the part's
// `limit`; a definition's inherited input and a body local reading it are open
// the same way.
package test {
	private import ScalarValues::*;

	constraint def Above {
		in limit : Integer;
		attribute margin : Integer = limit - 1;
		margin > 0
	}

	part def Rig {
		attribute limit : Integer = 5;
		constraint masked { in limit : Integer; limit > 4 }
		constraint direct { limit > 4 }
		constraint bound { in limit : Integer = Rig::limit; limit > 4 }
		constraint typed : Above;
		constraint typedBound : Above { in limit = Rig::limit; }
	}

	part def Bench {
		part rig : Rig;
		attribute maskedVerdict = rig.masked.result;
		attribute maskedLimit = rig.masked.limit;
		attribute directVerdict = rig.direct.result;
		attribute boundVerdict = rig.bound.result;
		attribute typedVerdict = rig.typed.result;
		attribute typedMargin = rig.typed.margin;
		attribute typedBoundVerdict = rig.typedBound.result;
		attribute typedBoundMargin = rig.typedBound.margin;
	}
}
