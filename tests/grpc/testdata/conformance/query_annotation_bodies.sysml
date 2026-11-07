package Model {
	metadata def m {
		attribute a default "a";
	}
	part <a> aaaa {
		doc /*Documentation*/
		@m {
			a := "aa";
		}
		port p;
	}
	part <b> bbb {
		rep language "rus"
			/*Textual representation*/
		port p;
	}
	interface connect (a.p, b.p);
}
