// The form a migrated RaiseExceptionAction writes: the raise is followed by a
// terminate action, and the step after the terminate never performs.
package test {
	private import ScalarValues::*;

	action def Raising {
		out reached : Integer = 0;

		first start;

		action raise {
			in exception : Integer = 1;
		}
		action 'terminate' terminate;
		action later {
			assign reached := 1;
		}

		first start then raise;
		first raise then 'terminate';
		first 'terminate' then later;
	}
}
