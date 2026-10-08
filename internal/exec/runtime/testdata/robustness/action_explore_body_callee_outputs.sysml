package test {
	private import ScalarValues::*;

	// Twice writes its output twice; the caller's pin takes each write as it is made.
	action def Twice {
		out value : Integer := 0;
		first start then w1;
		action w1 { assign value := 1; }
		then w2;
		action w2 { assign value := 2; }
		then done;
	}

	// reader, forked beside producer, may read its pin before, between or after
	// Twice's two writes, or before producer is performed at all, a typed error.
	action CalleeOutputs {
		attribute seen : Integer := 0;
		first start;
		then fork f;
			then producer;
			then reader;
		action producer : Twice;
		action reader { assign seen := producer.value; }
		succession producer then j;
		succession reader then j;
		join j;
		then done;
	}
}
