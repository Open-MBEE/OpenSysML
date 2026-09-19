// A `for` over a collection declared by the part enclosing it visits every
// element in order (1.0 + 2.0 + 3.5 = 6.5); the nested action is the entry
// behavior, named by its qualified path rather than copied to the top level.
package test {
	private import ScalarValues::*;

	part p {
		attribute steps : Real[*] = (1.0, 2.0, 3.5);

		action a {
			attribute total : Real = 0.0;
			attribute visited : Integer = 0;

			first start;

			action accumulate {
				for s in steps {
					assign total := total + s;
					assign visited := visited + 1;
				}
			}

			done;

			succession first start then accumulate;
			succession first accumulate then done;
		}
	}
}
