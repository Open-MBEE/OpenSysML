// A quantity is a ScalarQuantityValue, which no scalar number type generalizes
// or specializes: the run time refuses the quantity an untyped feature came to
// hold when it is written to a Real attribute, as `istype` denies it is a Real.
package test {
	action travel {
		attribute speed : ScalarValues::Real = 0.0;
		attribute measured;

		first start;

		action compute {
			assign measured := 10.0 [SI::m] / 2.0 [SI::s];
			then assign speed := measured;
		}

		done;

		succession first start then compute;
		succession first compute then done;
	}
}
