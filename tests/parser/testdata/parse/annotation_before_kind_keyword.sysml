package test {
	// An anonymous annotation ends with its body, so the kind keyword after
	// it opens the next member: `doc` is not a prefix of `attribute`.
	part def C {
		doc /* doc on the part def */
		attribute a : Real;
		comment /* a comment */
		port p : PW;
		locale "en_US" /* a localized comment */
		part sub : C;
		rep language "text" /* a representation */
		attribute b;
	}
	port def PW {
		doc /* doc on the port def */
		out item stuff;
	}
	part c : C {
		doc /* doc on the usage */
		attribute x;
	}
}
