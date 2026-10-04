// An end feature that may vary in time is implicitly constant (SysML 2.0
// §8.4.2.2), so an assignment to one once the performance is initialized is
// refused like a declared constant and the value stays.
package test {
	private import ScalarValues::*;

	action tune {
		end attribute level : Integer = 5;

		first start;

		action raise {
			assign level := 7;
		}

		done;

		succession first start then raise;
		succession first raise then done;
	}
}
