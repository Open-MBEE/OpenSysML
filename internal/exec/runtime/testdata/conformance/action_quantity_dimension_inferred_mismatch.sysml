// The value's dimension is only determined by evaluating it — `source` declares
// no quantity value type — so the type tier says nothing and the write itself
// refuses the speed the duration is handed.
package test {
	private import SI::*;
	private import ISQ::*;

	action travel {
		attribute source = 3.0 [m / s];
		attribute t : ISQSpaceTime::TimeValue = 0.0 [s];

		first start;

		action write {
			assign t := source;
		}

		done;

		succession first start then write;
		succession first write then done;
	}
}
