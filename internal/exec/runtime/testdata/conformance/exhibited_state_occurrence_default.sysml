// An exhibited machine starts from the occurrence's redefined slot value,
// rather than re-evaluating the definition's default.
package test {
	private import ScalarValues::*;

	state def Counting {
		attribute count : Integer = 1;

		entry; then active;
		state active {
			entry action increment {
				assign count := count + 1;
			}
		}
	}

	part def ConfiguredCounter {
		exhibit state modes : Counting {
			attribute redefines count = 5;
		}
	}
}
