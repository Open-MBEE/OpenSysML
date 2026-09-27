// `this` in a performed action written in its part names that part: an owned
// performance takes the object owning it as its context occurrence, so
// `that.<feature>` reads and writes that object's feature.
package test {
	private import ScalarValues::*;

	part def Host {
		attribute seen : Integer = 0;

		perform action marking {
			action step {
				assign that.seen := that.seen + 2;
			}
			first step;
		}
	}
}
