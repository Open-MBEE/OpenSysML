// Variation points whose choices interact: the trim a ring bears is chosen from
// its own variation point, and a variant of the nesting constrains it, so the
// consistent configurations are fewer than the combinations.
package test {
	private import ScalarValues::*;

	enum def Finish {
		enum polished;
		enum brushed;
	}

	attribute def Nesting;
	attribute def Trim;

	part def Ring {
		attribute nesting : Nesting;
		attribute trim : Trim;
		attribute finish : Finish;
	}

	part ringFamily : Ring {
		variation attribute :>> nesting {
			variant attribute nestingTrue;
			variant attribute nestingFalse;
		}

		variation attribute :>> trim {
			variant attribute plain;
			variant attribute gilded;
		}

		assert constraint variantsAgree {
			nesting == nesting::nestingTrue implies trim == trim::gilded
		}

		assert constraint gildedIsPolished {
			trim == trim::gilded implies finish == Finish::polished
		}
	}
}
