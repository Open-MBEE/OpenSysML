// The form a migrated RaiseExceptionAction writes: the raise is followed by a
// terminate action, so a later step never performs.
package test {
	private import ScalarValues::*;

	action def Raising {
		out reached : Integer = 0;

		first start;

		action raise {
			in exception : Integer = 1;
		}
		action 'terminate' terminate;
		action after {
			assign reached := 1;
		}
		action final terminate;

		first start then raise;
		first raise then 'terminate';
	}
}
