// A constraint's `in` parameter is the feature its body names (KerML §8.2.3.5): the owning part's feature
// spelled as it does not bind it, only a stated binding does; a body declaring none reads the context's.
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
