package Q {
	// `first a then j;` is the succession a -> j, however many are written: the
	// body's one start is `first start;`, and the positional `then` chain leaves
	// from the node declared nearest before it.
	private import ScalarValues::*;
	action def Main {
		out total : Integer = 0;
		action a assign total := total + 1;
		action b assign total := total + 10;
		action c assign total := total + 100;
		first start;
		then fork f;
			then a;
			then b;
		first a then j;
		first b then j;
		join j;
		first j then c;
		first c then done;
	}
}
