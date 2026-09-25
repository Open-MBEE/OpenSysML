package test {
	private import ScalarValues::*;

	// A `for` variable is a UsageDeclaration (SysML.xtext ForVariableDeclaration),
	// so it may state its type; the loop binds it per element regardless.
	action integrate {
		attribute total : Integer = 0;
		first start;

		action accumulate {
			for n : ScalarValues::Integer in (1, 2, 3) {
				assign total := total + n;
			}
		}

		done;

		succession first start then accumulate;
		succession first accumulate then done;
	}
}
