// A directly executed machine has no performance occurrence, so its declared
// attributes remain executor-local state data.
package test {
	private import ScalarValues::*;

	state def Counting {
		attribute count : Integer = 0;

		entry; then active;
		state active {
			entry action increment {
				assign count := count + 1;
			}
		}
	}
}
