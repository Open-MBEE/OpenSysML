// Anonymous inline action bodies as entry, do and exit behaviors, plus the
// named and empty forms.
package Bodies {
	state def Machine {
		attribute c : Integer = 0;
		state a {
			entry action {
				attribute k : Integer = 2;
				while k > 0 {
					assign c := c + k;
					assign k := k - 1;
				}
			}
			do action { assign c := c + 1; }
			exit action { }
			do action named { assign c := c + 1; }
		}
		state z;
		entry; then a;
		transition first a accept when true then z;
	}
}
