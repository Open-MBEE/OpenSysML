package test {
	private import ScalarValues::*;

	action def Base {
		attribute number : Integer = 0;
		first start;
		then action sender { send 5 to counter; }
		then action counter accept n : Integer;
		then action recorder { assign number := counter.n; }
		then done;
	}

	action def Direct :> Base;
}
