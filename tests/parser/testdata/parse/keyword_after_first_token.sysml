// Members whose second token is a kind keyword are prefixed declarations, a
// succession or a keyword-less feature (SysML.xtext PrefixMetadataMember,
// UsagePrefix, TargetSuccession, DefaultReferenceUsage): a name never stands
// ahead of the kind keyword, so `name keyword ...` is not a declaration.
package KeywordAfterFirstToken {
	metadata def M;
	attribute def T;
	part def Q;
	constraint def C;

	part def P {
		#M part p;
		#M item i : Q;
		ref part r : Q;
		abstract part a : Q;
		assert constraint c;
		assert not constraint n : C;
		x = 1;
		y : T = 2;
		z :> x;
		action def A {
			action s;
			first start;
			then s;
			then done;
		}
	}

	requirement def R {
		subject s : T;
		actor who : Q;
	}

	alias Part for Q;
	import Q::*;
	private import ScalarValues::Real;
}
