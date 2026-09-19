// A machine's declared attribute belongs to its exhibited state performance,
// not to a like-named attribute of the object exhibiting it.
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

	part def Counter {
		attribute count : Integer = 10;
		exhibit state modes : Counting;
	}
}
