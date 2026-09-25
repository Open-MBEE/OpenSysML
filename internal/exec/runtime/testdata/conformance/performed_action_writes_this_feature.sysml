// `this` in a performed action written in its part names that part: an owned
// performance takes the object owning it as its context occurrence, so
// `this.<feature>` reads and writes that object's feature.
package test {
	private import ScalarValues::*;

	part def Host {
		attribute seen : Integer = 0;

		perform action marking {
			action step {
				assign this.seen := this.seen + 2;
			}
			first step;
		}
	}
}
