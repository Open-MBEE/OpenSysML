// The arguments of an untyped action usage's invocation value replace the
// invoked definition's bound values, including one another input depends on:
// `z = 7` wins over `x * 2`.
package test {
	private import ScalarValues::*;

	action def Base {
		in x : Integer = 3;
		in z : Integer = x * 2;
		out r : Integer;

		first start;
		then assign r := z;
		then done;
	}

	action caller {
		out attribute result : Integer;

		first start;
		then action run = Base(x = 3, z = 7);
		then action keep { assign result := run.r; }
		then done;
	}
}
