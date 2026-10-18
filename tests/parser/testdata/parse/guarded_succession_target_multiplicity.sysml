package test {
	private import ScalarValues::*;

	action def A {
		attribute g : Boolean = true;
		first start then p;
		action p;
		succession first p if g then [*] a;
		action a[3];
		first p if g then [*] b;
		action b[2];
		then done;
	}
}
