// A constraint relating the variant a variation point selects to an enumeration
// value: both range over finitely many choices, which a translation encodes as
// datatype sorts rather than as numbers.
package test {
	enum def Finish {
		enum polished;
		enum brushed;
	}

	attribute def Nesting;

	part def Ring {
		attribute nesting : Nesting;
		attribute finish : Finish;
	}

	part ringFamily : Ring {
		variation attribute :>> nesting {
			variant attribute nestingTrue;
			variant attribute nestingFalse;
		}

		assert constraint finishMatchesNesting {
			nesting == nesting::nestingTrue implies finish == Finish::polished
		}
	}
}
